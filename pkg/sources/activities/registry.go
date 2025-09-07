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
	SummarizeActivity(ctx context.Context, act types.Activity) (*types.ActivitySummary, error)
}

type embedder interface {
	EmbedActivity(ctx context.Context, sum *types.ActivitySummary) ([]float32, error)
	EmbedActivityQuery(ctx context.Context, query string) ([]float32, error)
}

type activityStore interface {
	Upsert(ctx context.Context, act *types.DecoratedActivity) error
	Search(ctx context.Context, req types.SearchRequest) (*types.SearchResult, error)
}

// Create processes a single activity and stores it in the database.
// Returns false if activity was skipped.
func (r *Registry) Create(ctx context.Context, activity types.Activity, upsert bool) (bool, error) {
	// Check if activity already exists and has been processed
	if !upsert {
		result, err := r.activityRepo.Search(ctx, types.SearchRequest{
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
	embedding, err := r.embedder.EmbedActivity(ctx, summary)
	if err != nil {
		r.logger.Error().
			Err(err).
			Any("activity", activity).
			Any("summary", summary).
			Msg("compute embedding")
		return false, fmt.Errorf("compute embedding: %w", err)
	}

	err = r.activityRepo.Upsert(ctx, &types.DecoratedActivity{
		Activity:  activity,
		Summary:   summary,
		Embedding: embedding,
	})
	if err != nil {
		r.logger.Error().
			Err(err).
			Any("activity", activity).
			Msg("store activity")
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
		embedding, err := r.embedder.EmbedActivityQuery(ctx, req.Query)
		if err != nil {
			return nil, fmt.Errorf("compute query embedding: %w", err)
		}
		queryEmbedding = embedding
	}

	return r.activityRepo.Search(ctx, types.SearchRequest{
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
