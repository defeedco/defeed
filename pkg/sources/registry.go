package sources

import (
	"context"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"sort"
	"sync"

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
}

type sourceStore interface {
	Add(source Source) error
	Remove(uid string) error
	List() ([]Source, error)
	GetByID(uid string) (Source, error)
}

type activityStore interface {
	Add(activity *types.DecoratedActivity) error
	Remove(uid string) error
	List() ([]*types.DecoratedActivity, error)
}

type summarizer interface {
	Summarize(ctx context.Context, activity types.Activity) (*types.ActivitySummary, error)
}

func NewRegistry(
	logger *zerolog.Logger,
	summarizer summarizer,
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
	}

	r.startWorkers(1)

	return r
}

func (r *Registry) Add(source Source) error {
	existing, _ := r.sourceRepo.GetByID(source.UID())

	if existing != nil {
		return fmt.Errorf("source '%s' already exists", source.UID())
	}

	if err := source.Initialize(); err != nil {
		return fmt.Errorf("initialize source: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go source.Stream(ctx, r.activityQueue, r.errorQueue)

	err := r.sourceRepo.Add(source)
	if err != nil {
		cancel()
		return fmt.Errorf("add source: %w", err)
	}
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
			r.logger.Info().Msgf("Worker %d starting\n", workerID)
			for {
				select {
				case act := <-r.activityQueue:
					r.logger.Info().Msgf("[Worker %d] Processing activity %s\n", workerID, act.UID())

					summary, err := r.summarizer.Summarize(context.Background(), act)
					if err != nil {
						// TODO(pulse): Better way to handle errors here?
						//r.errorQueue <- fmt.Errorf("summarize activity: %w", err)
						r.logger.Error().Err(err).Msgf("[Worker %d] Error summarizing activity %v\n", workerID, err)
						continue
					}

					err = r.activityRepo.Add(&types.DecoratedActivity{
						Activity: act,
						Summary:  summary,
					})
					if err != nil {
						r.logger.Error().Err(err).Msgf("[Worker %d] Error storing activity %v\n", workerID, err)
					}

				case err := <-r.errorQueue:
					r.logger.Error().Err(err).Msgf("[Worker %d] Error processing activity %v\n", workerID, err)

				case <-r.done:
					r.logger.Info().Msgf("Worker %d shutting down\n", workerID)
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
