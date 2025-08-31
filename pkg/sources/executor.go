package sources

import (
	"context"
	"fmt"
	"sync"
	"time"

	sourcetypes "github.com/glanceapp/glance/pkg/sources/types"

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
	Add(source sourcetypes.Source) error
	Remove(uid string) error
	List() ([]sourcetypes.Source, error)
	GetByID(uid string) (sourcetypes.Source, error)
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

	// TODO(config): Move to config struct
	// Tweak the number of workers as needed.
	r.startWorkers(30)

	return r
}

func (r *Executor) Initialize() error {
	sources, err := r.activeSourceRepo.List()
	if err != nil {
		return fmt.Errorf("list sources: %w", err)
	}

	r.logger.Info().Int("count", len(sources)).Msg("Initializing sources")

	ctx := context.Background()
	for _, source := range sources {
		sLogger := sourceLogger(source, r.logger)
		if err := source.Initialize(ctx, sLogger); err != nil {
			sLogger.Error().
				Err(err).
				Msg("Failed to initialize source")
			continue
		}

		activities, err := r.activityRepo.Search(types.SearchRequest{
			SourceUIDs: []types.TypedUID{source.UID()},
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

		// Do not block the initialization since the result/error reporting is async
		go r.executeSourceOnce(source, since)
		r.scheduleSource(source)

		sLogger.Info().Msg("Source initialized")
	}

	r.logger.Info().Msg("Source initialization complete")
	return nil
}

func (r *Executor) scheduleSource(source sourcetypes.Source) {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancelBySourceID.Store(source.UID(), cancel)

	go func() {
		ticker := r.getSourceTicker(source)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				activities, err := r.activityRepo.Search(types.SearchRequest{
					SourceUIDs: []types.TypedUID{source.UID()},
					Limit:      1,
					SortBy:     types.SortByDate,
				})
				if err != nil {
					r.logger.Error().
						Str("source_id", source.UID().String()).
						Err(err).Msg("Failed to search activities for scheduling")
					continue
				}

				logEvent := r.logger.Debug()
				var since types.Activity = nil
				if len(activities) > 0 {
					since = activities[0].Activity
					logEvent.Str("last_activity_uid", since.UID().String())
				}
				logEvent.Msg("Polling source")

				r.executeSourceOnce(source, since)
			}
		}
	}()
}

func (r *Executor) executeSourceOnce(source sourcetypes.Source, since types.Activity) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	activityChan := make(chan types.Activity, 100)
	errorChan := make(chan error, 100)

	go func() {
		defer close(activityChan)
		defer close(errorChan)
		source.Stream(ctx, since, activityChan, errorChan)
	}()

	for {
		select {
		case activity, ok := <-activityChan:
			if !ok {
				return
			}
			r.activityQueue <- activity
		case err, ok := <-errorChan:
			if !ok {
				return
			}
			r.errorQueue <- err
		case <-ctx.Done():
			return
		}
	}
}

func (r *Executor) getSourceTicker(source sourcetypes.Source) *time.Ticker {
	// Default to 30 minutes for all sources
	// TODO: Make this configurable per source type?
	return time.NewTicker(30 * time.Minute)
}

// Add starts processing activities from the source.
func (r *Executor) Add(source sourcetypes.Source) error {
	existing, _ := r.activeSourceRepo.GetByID(source.UID().String())

	if existing != nil {
		// source already exists, we don't need to do anything
		return nil
	}

	if err := source.Initialize(context.Background(), sourceLogger(source, r.logger)); err != nil {
		return fmt.Errorf("initialize source: %w", err)
	}

	err := r.activeSourceRepo.Add(source)
	if err != nil {
		return fmt.Errorf("add source: %w", err)
	}

	// Set to nil since there are no previous activities for this source yet.
	var since types.Activity = nil
	r.executeSourceOnce(source, since)
	r.scheduleSource(source)

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

func (r *Executor) Search(ctx context.Context, query string, sourceUIDs []types.TypedUID, minSimilarity float32, limit int, sortBy types.SortBy, period types.Period) ([]*types.DecoratedActivity, error) {
	req := types.SearchRequest{
		SourceUIDs:    sourceUIDs,
		MinSimilarity: minSimilarity,
		Limit:         limit,
		SortBy:        sortBy,
		Period:        period,
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

func sourceLogger(source sourcetypes.Source, logger *zerolog.Logger) *zerolog.Logger {
	out := logger.With().
		Str("source_type", source.UID().Type()).
		Str("source_uid", source.UID().String()).
		Logger()

	return &out
}

func activityLogger(activity types.Activity, logger *zerolog.Logger) *zerolog.Logger {
	out := logger.With().
		Str("activity_id", activity.UID().String()).
		Str("source_type", activity.UID().Type()).
		Str("source_uid", activity.SourceUID().String()).
		Logger()

	return &out
}
