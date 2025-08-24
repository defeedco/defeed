package sources

import (
	"context"
	"fmt"
	"sync"

	types2 "github.com/glanceapp/glance/pkg/sources/types"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/rs/zerolog"
)

// Executor manages the execution of active sources.
//
// It is responsible for:
// - Managing the lifecycle of sources
// - Fetching activities from the sources
// - Summarizing & embedding activities
// - Storing activities in the database
// - Retrieving stored activities
type Executor struct {
	activeSourceRepo sourceStore
	activityRepo     activityStore

	cancelBySourceID sync.Map
	activityQueue    chan types.Activity
	errorQueue       chan error
	done             chan struct{}

	logger     *zerolog.Logger
	summarizer summarizer
	embedder   embedder
}

type sourceStore interface {
	Add(source types2.Source) error
	Remove(uid string) error
	List() ([]types2.Source, error)
	GetByID(uid string) (types2.Source, error)
}

type activityStore interface {
	Upsert(activity *types.DecoratedActivity) error
	Remove(uid string) error
	List() ([]*types.DecoratedActivity, error)
	Search(req types.SearchRequest) ([]*types.DecoratedActivity, error)
}

type summarizer interface {
	Summarize(ctx context.Context, activity types.Activity) (*types.ActivitySummary, error)
	SummarizeMany(ctx context.Context, activities []*types.DecoratedActivity, query string) (*types.ActivitiesSummary, error)
}

type embedder interface {
	Embed(ctx context.Context, summary *types.ActivitySummary) ([]float32, error)
}

func NewExecutor(
	logger *zerolog.Logger,
	summarizer summarizer,
	embedder embedder,
	activityRepo activityStore,
	sourceRepo sourceStore,
) *Executor {
	r := &Executor{
		activityRepo:     activityRepo,
		activeSourceRepo: sourceRepo,
		activityQueue:    make(chan types.Activity),
		errorQueue:       make(chan error),
		done:             make(chan struct{}),
		logger:           logger,
		summarizer:       summarizer,
		embedder:         embedder,
	}

	// Tweak the number of workers as needed.
	r.startWorkers(10)

	return r
}

func (r *Executor) Initialize() error {
	sources, err := r.activeSourceRepo.List()
	if err != nil {
		return fmt.Errorf("list sources: %w", err)
	}

	r.logger.Info().Int("count", len(sources)).Msg("Initializing sources")

	for _, source := range sources {
		sLogger := sourceLogger(source, r.logger)
		if err := source.Initialize(sLogger); err != nil {
			sLogger.Error().
				Err(err).
				Msg("Failed to initialize source")
			continue
		}

		activities, err := r.activityRepo.Search(types.SearchRequest{
			SourceUIDs: []string{source.UID()},
			Limit:      1,
			SortBy:     types.SortByDate,
		})
		if err != nil {
			return fmt.Errorf("search existing activities: %w", err)
		}

		var since types.Activity = nil
		if len(activities) > 0 {
			since = activities[0].Activity
		}

		ctx, cancel := context.WithCancel(context.Background())
		go source.Stream(ctx, since, r.activityQueue, r.errorQueue)

		r.cancelBySourceID.Store(source.UID(), cancel)

		sLogger.Info().Msg("Source initialized")
	}

	r.logger.Info().Msg("Source initialization complete")
	return nil
}

// Add starts processing activities from the source.
func (r *Executor) Add(source types2.Source) error {
	existing, _ := r.activeSourceRepo.GetByID(source.UID())

	if existing != nil {
		// source already exists, we don't need to do anything
		return nil
	}

	if err := source.Initialize(sourceLogger(source, r.logger)); err != nil {
		return fmt.Errorf("initialize source: %w", err)
	}

	err := r.activeSourceRepo.Add(source)
	if err != nil {
		return fmt.Errorf("add source: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Set to nil since there are no previous activities for this source yet.
	var since types.Activity = nil
	go source.Stream(ctx, since, r.activityQueue, r.errorQueue)

	r.cancelBySourceID.Store(source.UID(), cancel)

	return nil
}

// Remove stops the source execution
func (r *Executor) Remove(uid string) error {
	existing, _ := r.activeSourceRepo.GetByID(uid)

	if existing != nil {
		return fmt.Errorf("source '%s' not found", uid)
	}

	cancel, ok := r.cancelBySourceID.Load(uid)
	if !ok {
		return fmt.Errorf("source '%s' cancel func not found", uid)
	}
	cancel.(context.CancelFunc)()
	r.cancelBySourceID.Delete(uid)

	err := r.activeSourceRepo.Remove(uid)
	if err != nil {
		return fmt.Errorf("remove source: %w", err)
	}

	return nil
}

func (r *Executor) startWorkers(nWorkers int) {
	var wg sync.WaitGroup

	for i := range nWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			wLogger := r.logger.With().Int("worker_id", workerID).Logger()

			wLogger.Info().Msg("Worker started")
			for {
				select {
				case act := <-r.activityQueue:

					actLogger := activityLogger(act, &wLogger)

					actLogger.Debug().Msg("Processing activity")

					summary, err := r.summarizer.Summarize(context.Background(), act)
					if err != nil {
						actLogger.Error().
							Err(err).
							Msg("Error summarizing activity")
						continue
					}

					// Compute embedding for the full summary
					embedding, err := r.embedder.Embed(context.Background(), summary)
					if err != nil {
						actLogger.Error().
							Err(err).
							Msg("Error computing embedding")
						continue
					}

					err = r.activityRepo.Upsert(&types.DecoratedActivity{
						Activity:  act,
						Summary:   summary,
						Embedding: embedding,
					})
					if err != nil {
						actLogger.Error().
							Err(err).
							Msg("Error storing activity")
					}

				case err := <-r.errorQueue:
					// TODO: Decorate errors with source ID
					wLogger.Error().
						Err(err).
						Msg("Error from source")

				case <-r.done:
					wLogger.Info().Msg("Worker shutting down")
					return
				}
			}
		}(i + 1)
	}
}

func (r *Executor) Shutdown() {
	close(r.done)

	r.cancelBySourceID.Range(func(key, value interface{}) bool {
		cancel := value.(context.CancelFunc)
		cancel()
		return true
	})
	r.cancelBySourceID.Clear()
}

func (r *Executor) Search(ctx context.Context, query string, sourceUIDs []string, minSimilarity float32, limit int, sortBy types.SortBy) ([]*types.DecoratedActivity, error) {
	req := types.SearchRequest{
		SourceUIDs:    sourceUIDs,
		MinSimilarity: minSimilarity,
		Limit:         limit,
		SortBy:        sortBy,
	}

	if query != "" {
		embedding, err := r.embedder.Embed(ctx, &types.ActivitySummary{
			FullSummary: query,
		})
		if err != nil {
			return nil, fmt.Errorf("compute query embedding: %w", err)
		}
		req.QueryEmbedding = embedding
	}

	return r.activityRepo.Search(req)
}

func sourceLogger(source types2.Source, logger *zerolog.Logger) *zerolog.Logger {
	out := logger.With().
		Str("source_type", source.Type()).
		Str("source_uid", source.UID()).
		Logger()

	return &out
}

func activityLogger(activity types.Activity, logger *zerolog.Logger) *zerolog.Logger {
	out := logger.With().
		Str("activity_id", activity.UID()).
		Str("source_type", activity.SourceType()).
		Str("source_uid", activity.SourceUID()).
		Logger()

	return &out
}
