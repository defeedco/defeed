package activities

import (
	"context"
	"fmt"
	"sync"

	"github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/rs/zerolog"
)

type Registry struct {
	activityRepo activityStore
	logger       *zerolog.Logger
	summarizer   summarizer
	embedder     embedder
	// activityLocks provides per-activity ID locking to prevent race conditions
	activityLocks sync.Map // map[string]*sync.Mutex
}

func NewRegistry(
	logger *zerolog.Logger,
	activityRepo activityStore,
	summarizer summarizer,
	embedder embedder,
) *Registry {
	return &Registry{
		activityRepo: activityRepo,
		logger:       logger,
		summarizer:   summarizer,
		embedder:     embedder,
	}
}

type summarizer interface {
	SummarizeActivity(ctx context.Context, act types.Activity) (*types.ActivitySummary, error)
}

type embedder interface {
	EmbedActivity(ctx context.Context, act types.Activity, summary *types.ActivitySummary) ([]float32, error)
	EmbedActivityQuery(ctx context.Context, query string) ([]float32, error)
}

type activityStore interface {
	Upsert(ctx context.Context, act *types.DecoratedActivity) error
	Search(ctx context.Context, req types.SearchRequest) (*types.SearchResult, error)
}

type CreateRequest struct {
	Activity types.Activity
	// ForceReprocessSummary recomputes the short/full summary.
	// If ForceReprocessSummary is true, Upsert must also be true.
	ForceReprocessSummary bool
	// ForceReprocessEmbedding recomputes the embedding.
	// If ForceReprocessEmbedding is true, Upsert must also be true.
	ForceReprocessEmbedding bool
	// Upsert updates the existing record.
	Upsert bool
}

// Create processes a single activity and stores it in the database.
// Returns false if activity was skipped.
func (r *Registry) Create(ctx context.Context, req CreateRequest) (bool, error) {
	if req.ForceReprocessSummary && !req.Upsert {
		return false, fmt.Errorf("reprocess summary without upsert is not allowed")
	}
	if req.ForceReprocessEmbedding && !req.Upsert {
		return false, fmt.Errorf("reprocess embedding without upsert is not allowed")
	}

	// Race conditions can occur if multiple goroutines process the same activity concurrently.
	lockKey := req.Activity.UID().String()
	lock, _ := r.activityLocks.LoadOrStore(lockKey, &sync.Mutex{})
	mu := lock.(*sync.Mutex)

	mu.Lock()
	defer func() {
		mu.Unlock()
		r.activityLocks.Delete(lockKey)
	}()

	existing, err := r.findOne(ctx, req.Activity.UID())
	if err != nil {
		return false, fmt.Errorf("load existing activity: %w", err)
	}

	if existing != nil && !req.Upsert {
		return false, nil
	}

	var summary *types.ActivitySummary
	var embedding []float32

	if existing != nil {
		summary = existing.Summary
		embedding = existing.Embedding
	}

	if req.ForceReprocessSummary || existing == nil || existing.Summary.FullSummary == "" || existing.Summary.ShortSummary == "" {
		summary, err = r.summarizer.SummarizeActivity(ctx, req.Activity)
		if err != nil {
			return false, fmt.Errorf("summarize activity: %w", err)
		}
	}

	if req.ForceReprocessEmbedding || existing == nil || len(existing.Embedding) == 0 {
		embedding, err = r.embedder.EmbedActivity(ctx, req.Activity, summary)
		if err != nil {
			return false, fmt.Errorf("compute embedding: %w", err)
		}
	}

	err = r.activityRepo.Upsert(ctx, &types.DecoratedActivity{
		Activity:  req.Activity,
		Summary:   summary,
		Embedding: embedding,
	})
	if err != nil {
		return false, fmt.Errorf("upsert activity: %w", err)
	}

	return true, nil
}

func (r *Registry) findOne(ctx context.Context, uid types.TypedUID) (*types.DecoratedActivity, error) {
	res, err := r.activityRepo.Search(ctx, types.SearchRequest{
		ActivityUIDs: []types.TypedUID{uid},
		Limit:        1,
	})
	if err != nil {
		return nil, fmt.Errorf("find one: %w", err)
	}
	if len(res.Activities) == 0 {
		return nil, nil
	}
	return res.Activities[0], nil
}

type SearchRequest struct {
	Query         string
	ActivityUIDs  []types.TypedUID
	SourceUIDs    []types.TypedUID
	MinSimilarity float32
	Limit         int
	Cursor        string
	SortBy        types.SortBy
	Period        types.Period
}

func (r *Registry) Search(ctx context.Context, req SearchRequest) (*types.SearchResult, error) {
	var queryEmbedding []float32
	if req.Query != "" {
		embedding, err := r.embedder.EmbedActivityQuery(ctx, req.Query)
		if err != nil {
			return nil, fmt.Errorf("compute query embedding: %w", err)
		}
		queryEmbedding = embedding
	}

	recencyWeight := 0.0
	// We only care about recency when viewing today's activities.
	if req.Period == types.PeriodDay {
		recencyWeight = 1
	}

	return r.activityRepo.Search(ctx, types.SearchRequest{
		SourceUIDs:        req.SourceUIDs,
		ActivityUIDs:      req.ActivityUIDs,
		MinSimilarity:     req.MinSimilarity,
		Limit:             req.Limit,
		Cursor:            req.Cursor,
		SortBy:            req.SortBy,
		Period:            req.Period,
		QueryEmbedding:    queryEmbedding,
		SocialScoreWeight: 2,
		SimilarityWeight:  4,
		RecencyWeight:     recencyWeight,
	})
}
