package activities

import (
	"context"
	"fmt"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/rs/zerolog"
)

type Registry struct {
	activityRepo activityStore
	logger       *zerolog.Logger
	summarizer   summarizer
	embedder     embedder
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
	SummarizeActivity(ctx context.Context, activity types.Activity) (*types.ActivitySummary, error)
}

type embedder interface {
	Embed(ctx context.Context, summary *types.ActivitySummary) ([]float32, error)
}

type activityStore interface {
	Upsert(activity *types.DecoratedActivity) error
	Remove(uid string) error
	List() ([]*types.DecoratedActivity, error)
	Search(req types.SearchRequest) (*types.SearchResult, error)
}

// Create processes a single activity and stores it in the database.
// Returns false if activity was skipped.
func (r *Registry) Create(ctx context.Context, activity types.Activity, upsert bool) (bool, error) {
	// Check if activity already exists and has been processed
	if !upsert {
		result, err := r.activityRepo.Search(types.SearchRequest{
			ActivityUIDs: []types.TypedUID{activity.UID()},
			Limit:        1,
		})
		if err != nil {
			return false, fmt.Errorf("check if activity exists: %w", err)
		}

		// Skip processing if activity already exists and has been processed
		if len(result.Activities) > 0 {
			existing := result.Activities[0]
			if existing.Summary != nil && existing.Summary.FullSummary != "" && len(existing.Embedding) > 0 {
				return false, nil
			}
		}
	}

	// Summarize activity
	summary, err := r.summarizer.SummarizeActivity(ctx, activity)
	if err != nil {
		return false, fmt.Errorf("summarize activity: %w", err)
	}

	// Compute embedding for the summary
	embedding, err := r.embedder.Embed(ctx, summary)
	if err != nil {
		return false, fmt.Errorf("compute embedding: %w", err)
	}

	err = r.activityRepo.Upsert(&types.DecoratedActivity{
		Activity:  activity,
		Summary:   summary,
		Embedding: embedding,
	})
	if err != nil {
		return false, fmt.Errorf("store activity: %w", err)
	}

	return true, nil
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
		embedding, err := r.embedder.Embed(ctx, &types.ActivitySummary{
			FullSummary: req.Query,
		})
		if err != nil {
			return nil, fmt.Errorf("compute query embedding: %w", err)
		}
		queryEmbedding = embedding
	}

	return r.activityRepo.Search(types.SearchRequest{
		SourceUIDs:     req.SourceUIDs,
		ActivityUIDs:   req.ActivityUIDs,
		MinSimilarity:  req.MinSimilarity,
		Limit:          req.Limit,
		Cursor:         req.Cursor,
		SortBy:         req.SortBy,
		Period:         req.Period,
		QueryEmbedding: queryEmbedding,
	})
}
