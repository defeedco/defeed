package sources

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/glanceapp/glance/pkg/sources/activities"

	activitytypes "github.com/glanceapp/glance/pkg/sources/activities/types"
	sourcetypes "github.com/glanceapp/glance/pkg/sources/types"

	"github.com/rs/zerolog"
)

// Scheduler manages the execution of active sources.
//
// It is responsible for:
// - Initializing sources
// - Managing the lifecycle of polling sources
// - Scheduling the processing of activities
type Scheduler struct {
	activeSourceRepo   sourceStore
	activityRegistry   *activities.Registry
	activityWorkerPool pond.Pool
	cancelBySourceID   sync.Map
	cancelByActivityID sync.Map
	logger             *zerolog.Logger
	sourceConfig       *sourcetypes.ProviderConfig
}

type sourceStore interface {
	Add(source sourcetypes.Source) error
	Remove(uid string) error
	List() ([]sourcetypes.Source, error)
	GetByID(uid string) (sourcetypes.Source, error)
}

func NewScheduler(
	logger *zerolog.Logger,
	sourceRepo sourceStore,
	activityRegistry *activities.Registry,
	config *Config,
	sourceConfig *sourcetypes.ProviderConfig,
) *Scheduler {
	return &Scheduler{
		activeSourceRepo:   sourceRepo,
		activityRegistry:   activityRegistry,
		logger:             logger,
		activityWorkerPool: pond.NewPool(config.MaxActivityProcessorConcurrency),
		sourceConfig:       sourceConfig,
	}
}

func (r *Scheduler) Initialize(ctx context.Context) error {
	sources, err := r.activeSourceRepo.List()
	if err != nil {
		return fmt.Errorf("list sources: %w", err)
	}

	r.logger.Info().Int("count", len(sources)).Msg("Initializing sources")

	for _, source := range sources {
		sLogger := sourceLogger(source, r.logger)
		if err := source.Initialize(sLogger, r.sourceConfig); err != nil {
			sLogger.Error().
				Err(err).
				Msg("Failed to initialize source")
			continue
		}

		result, err := r.activityRegistry.Search(ctx, activities.SearchRequest{
			SourceUIDs: []activitytypes.TypedUID{source.UID()},
			Limit:      1,
			SortBy:     activitytypes.SortByDate,
		})
		if err != nil {
			return fmt.Errorf("search existing activities: %w", err)
		}

		var since activitytypes.Activity = nil
		if len(result.Activities) > 0 {
			since = result.Activities[0].Activity
		}

		// Do not block the initialization since the result/error reporting is async
		go r.executeSourceOnce(source, since)
		r.scheduleSource(source)

		sLogger.Info().Msg("Source initialized")
	}

	r.logger.Info().Msg("Source initialization complete")
	return nil
}

func (r *Scheduler) scheduleSource(source sourcetypes.Source) {
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
				result, err := r.activityRegistry.Search(ctx, activities.SearchRequest{
					SourceUIDs: []activitytypes.TypedUID{source.UID()},
					Limit:      1,
					SortBy:     activitytypes.SortByDate,
				})
				if err != nil {
					r.logger.Error().
						Str("source_id", source.UID().String()).
						Err(err).Msg("Failed to search activities for scheduling")
					continue
				}

				logEvent := r.logger.Debug()
				var since activitytypes.Activity = nil
				if len(result.Activities) > 0 {
					since = result.Activities[0].Activity
					logEvent.Str("last_activity_uid", since.UID().String())
				}
				logEvent.Msg("Polling source")

				r.executeSourceOnce(source, since)
			}
		}
	}()
}

func (r *Scheduler) executeSourceOnce(source sourcetypes.Source, since activitytypes.Activity) {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancelBySourceID.Store(source.UID(), cancel)

	activityChan := make(chan activitytypes.Activity, 100)
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
				activityChan = nil
			} else {
				r.processActivity(activity)
			}
		case err, ok := <-errorChan:
			if !ok {
				errorChan = nil
			} else {
				r.logger.Error().
					Err(err).
					Str("source_id", source.UID().String()).
					Msg("Poll activities error")
			}
		case <-ctx.Done():
			return
		}

		// Exit when both channels are closed
		if activityChan == nil && errorChan == nil {
			return
		}
	}
}

func (r *Scheduler) processActivity(activity activitytypes.Activity) {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancelByActivityID.Store(activity.UID(), cancel)

	r.activityWorkerPool.Submit(func() {
		// Do not force reprocessing if activity already exists
		isCreated, err := r.activityRegistry.Create(ctx, activity, false)
		if err != nil {
			// TODO: Better error handling (retry or track the failures)
			r.logger.Error().
				Err(err).
				Str("activity_uid", activity.UID().String()).
				Msg("Failed to create activity")
		}

		r.logger.Debug().
			Str("activity_uid", activity.UID().String()).
			Bool("created", isCreated).
			Msg("Activity processed")
	})

}

func (r *Scheduler) getSourceTicker(source sourcetypes.Source) *time.Ticker {
	// Default to 2 hours for all sources
	// TODO: Make this configurable per source type?
	return time.NewTicker(2 * time.Hour)
}

// Add starts processing activities from the source.
func (r *Scheduler) Add(source sourcetypes.Source) error {
	existing, _ := r.activeSourceRepo.GetByID(source.UID().String())

	if existing != nil {
		// source already exists, we don't need to do anything
		return nil
	}

	if err := source.Initialize(sourceLogger(source, r.logger), r.sourceConfig); err != nil {
		return fmt.Errorf("initialize source: %w", err)
	}

	err := r.activeSourceRepo.Add(source)
	if err != nil {
		return fmt.Errorf("add source: %w", err)
	}

	// Set to nil since there are no previous activities for this source yet.
	var since activitytypes.Activity = nil
	go r.executeSourceOnce(source, since)
	r.scheduleSource(source)

	return nil
}

// Remove stops the source execution
func (r *Scheduler) Remove(uid string) error {
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

func (r *Scheduler) Shutdown() {
	// Cancel source scheduling
	r.cancelBySourceID.Range(func(key, value interface{}) bool {
		cancel := value.(context.CancelFunc)
		cancel()
		return true
	})
	r.cancelBySourceID.Clear()

	// Cancel processing activities
	r.cancelByActivityID.Range(func(key, value interface{}) bool {
		cancel := value.(context.CancelFunc)
		cancel()
		return true
	})
	r.cancelByActivityID.Clear()
}

type ListRequest struct {
	SourceUIDs []activitytypes.TypedUID
}

func (r *Scheduler) List(req ListRequest) ([]sourcetypes.Source, error) {
	result, err := r.activeSourceRepo.List()
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}

	includeSourceUIDs := make(map[string]bool)
	for _, uid := range req.SourceUIDs {
		includeSourceUIDs[uid.String()] = true
	}

	var filtered []sourcetypes.Source
	if len(includeSourceUIDs) > 0 {
		for _, source := range result {
			if includeSourceUIDs[source.UID().String()] {
				filtered = append(filtered, source)
			}
		}
	} else {
		filtered = result
	}

	return filtered, nil
}

func sourceLogger(source sourcetypes.Source, logger *zerolog.Logger) *zerolog.Logger {
	out := logger.With().
		Str("source_type", source.UID().Type()).
		Str("source_uid", source.UID().String()).
		Logger()

	return &out
}
