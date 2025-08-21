package sources

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/rs/zerolog"
)

type Registry struct {
	sourceRepo   sourceStore
	activityRepo activityStore

	cancelBySourceID sync.Map
	activityQueue    chan types.Activity
	errorQueue       chan error
	done             chan struct{}

	logger     *zerolog.Logger
	summarizer summarizer
	embedder   embedder
}

type sourceStore interface {
	Add(source Source) error
	Remove(uid string) error
	List() ([]Source, error)
	GetByID(uid string) (Source, error)
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

func NewRegistry(
	logger *zerolog.Logger,
	summarizer summarizer,
	embedder embedder,
	activityRepo activityStore,
	sourceRepo sourceStore,
) *Registry {
	r := &Registry{
		activityRepo:  activityRepo,
		sourceRepo:    sourceRepo,
		activityQueue: make(chan types.Activity),
		errorQueue:    make(chan error),
		done:          make(chan struct{}),
		logger:        logger,
		summarizer:    summarizer,
		embedder:      embedder,
	}

	// Tweak the number of workers as needed.
	r.startWorkers(10)

	return r
}

func (r *Registry) Initialize() error {
	sources, err := r.sourceRepo.List()
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
			since = activities[0]
		}

		ctx, cancel := context.WithCancel(context.Background())
		go source.Stream(ctx, since, r.activityQueue, r.errorQueue)

		r.cancelBySourceID.Store(source.UID(), cancel)

		sLogger.Info().Msg("Source initialized")
	}

	r.logger.Info().Msg("Source initialization complete")
	return nil
}

func (r *Registry) Add(source Source) error {
	existing, _ := r.sourceRepo.GetByID(source.UID())

	if existing != nil {
		// source already exists, we don't need to do anything
		return nil
	}

	if err := source.Initialize(sourceLogger(source, r.logger)); err != nil {
		return fmt.Errorf("initialize source: %w", err)
	}

	err := r.sourceRepo.Add(source)
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

func (r *Registry) Remove(uid string) error {
	existing, _ := r.sourceRepo.GetByID(uid)

	if existing != nil {
		return fmt.Errorf("source '%s' not found", uid)
	}

	cancel, ok := r.cancelBySourceID.Load(uid)
	if !ok {
		return fmt.Errorf("source '%s' cancel func not found", uid)
	}
	cancel.(context.CancelFunc)()
	r.cancelBySourceID.Delete(uid)

	err := r.sourceRepo.Remove(uid)
	if err != nil {
		return fmt.Errorf("remove source: %w", err)
	}

	return nil
}

func (r *Registry) Sources() ([]Source, error) {
	return r.sourceRepo.List()
}

func (r *Registry) Source(uid string) (Source, error) {
	return r.sourceRepo.GetByID(uid)
}

func (r *Registry) Activities() ([]*types.DecoratedActivity, error) {
	matches, err := r.activityRepo.List()
	if err != nil {
		return nil, fmt.Errorf("repo list: %w", err)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].CreatedAt().Before(matches[j].CreatedAt())
	})

	return matches, nil
}

func (r *Registry) ActivitiesBySource(sourceUID string) ([]*types.DecoratedActivity, error) {
	activities, err := r.Activities()
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}

	matches := make([]*types.DecoratedActivity, 0)
	for _, a := range activities {
		if a.SourceUID() == sourceUID {
			matches = append(matches, a)
		}
	}

	return matches, nil
}

func (r *Registry) startWorkers(nWorkers int) {
	var wg sync.WaitGroup

	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			r.logger.Info().Int("worker_id", workerID).Msg("Worker started")
			for {
				select {
				case act := <-r.activityQueue:

					actLogger := activityLogger(act, r.logger).With().
						Int("worker_id", workerID).
						Logger()

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
					r.logger.Error().
						Err(err).
						Int("worker_id", workerID).
						Msg("Error from source")

				case <-r.done:
					r.logger.Info().Int("worker_id", workerID).Msg("Worker shutting down")
					return
				}
			}
		}(i + 1)
	}
}

func (r *Registry) Shutdown() {
	close(r.done)

	r.cancelBySourceID.Range(func(key, value interface{}) bool {
		cancel := value.(context.CancelFunc)
		cancel()
		return true
	})
	r.cancelBySourceID.Clear()
}

func (r *Registry) Search(ctx context.Context, query string, sourceUIDs []string, minSimilarity float32, limit int, sortBy types.SortBy) ([]*types.DecoratedActivity, error) {
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

func (r *Registry) Summary(ctx context.Context, query string, sourceUIDs []string) (*types.ActivitiesSummary, error) {
	activities, err := r.Search(ctx, query, sourceUIDs, 0.0, 20, types.SortBySimilarity)
	if err != nil {
		return nil, fmt.Errorf("search activities: %w", err)
	}

	return r.summarizer.SummarizeMany(ctx, activities, query)
}

func sourceLogger(source Source, logger *zerolog.Logger) *zerolog.Logger {
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
