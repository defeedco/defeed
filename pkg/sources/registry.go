package sources

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/glanceapp/glance/pkg/sources/common"
	"github.com/rs/zerolog"
)

type Registry struct {
	sources         map[string]cancelableSource
	sourcesMutex    sync.Mutex
	activities      map[string]DecoratedActivity
	activitiesMutex sync.Mutex

	activityQueue chan common.Activity
	errorQueue    chan error
	done          chan struct{}

	logger     *zerolog.Logger
	summarizer summarizer
}

type summarizer interface {
	Summarize(ctx context.Context, activity common.Activity) (*common.ActivitySummary, error)
}

type DecoratedActivity struct {
	common.Activity
	Summary *common.ActivitySummary
}

type cancelableSource struct {
	Source
	cancel context.CancelFunc
}

func NewRegistry(logger *zerolog.Logger, summarizer summarizer) *Registry {
	r := &Registry{
		sources:       make(map[string]cancelableSource),
		activities:    make(map[string]DecoratedActivity),
		activityQueue: make(chan common.Activity),
		errorQueue:    make(chan error),
		done:          make(chan struct{}),
		logger:        logger,
		summarizer:    summarizer,
	}

	r.startWorkers(1)

	return r
}

func (r *Registry) Add(source Source) error {
	r.sourcesMutex.Lock()
	defer r.sourcesMutex.Unlock()

	if _, exists := r.sources[source.UID()]; exists {
		return fmt.Errorf("source '%s' already exists", source.UID())
	}

	if err := source.Initialize(); err != nil {
		return fmt.Errorf("initialize source: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go source.Stream(ctx, r.activityQueue, r.errorQueue)

	r.sources[source.UID()] = cancelableSource{
		Source: source,
		cancel: cancel,
	}

	return nil
}

func (r *Registry) Remove(uid string) error {
	r.sourcesMutex.Lock()
	defer r.sourcesMutex.Unlock()

	h, ok := r.sources[uid]
	if !ok {
		return fmt.Errorf("source '%s' not found", uid)
	}

	h.cancel()
	delete(r.sources, uid)

	return nil
}

func (r *Registry) Sources() ([]Source, error) {
	r.sourcesMutex.Lock()
	defer r.sourcesMutex.Unlock()

	out := make([]Source, 0, len(r.sources))
	for _, s := range r.sources {
		out = append(out, s.Source)
	}
	return out, nil
}

func (r *Registry) Source(uid string) (Source, error) {
	r.sourcesMutex.Lock()
	defer r.sourcesMutex.Unlock()

	s, ok := r.sources[uid]
	if !ok {
		return nil, fmt.Errorf("source '%s' not found", uid)
	}

	return s.Source, nil
}

func (r *Registry) Activities() ([]DecoratedActivity, error) {
	r.activitiesMutex.Lock()
	defer r.activitiesMutex.Unlock()

	matches := make([]DecoratedActivity, 0)
	for _, a := range r.activities {
		matches = append(matches, a)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].CreatedAt().Before(matches[j].CreatedAt())
	})

	return matches, nil
}

func (r *Registry) ActivitiesBySource(sourceUID string) ([]DecoratedActivity, error) {
	r.activitiesMutex.Lock()
	defer r.activitiesMutex.Unlock()

	matches := make([]DecoratedActivity, 0)
	for _, a := range r.activities {
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
						//r.errorQueue <- fmt.Errorf("summarize activity: %w", err)
						r.logger.Error().Err(err).Msgf("[Worker %d] Error summarizing activity %v\n", workerID, err)
						continue
					}

					r.activitiesMutex.Lock()
					r.activities[act.UID()] = DecoratedActivity{
						Activity: act,
						Summary:  summary,
					}
					r.activitiesMutex.Unlock()

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

	r.sourcesMutex.Lock()
	for _, source := range r.sources {
		source.cancel()
	}
	r.sourcesMutex.Unlock()
}
