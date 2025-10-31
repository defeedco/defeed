package nlp

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/embeddings"

	"github.com/defeedco/defeed/pkg/sources/activities/types"
)

type ActivityEmbedder struct {
	embedder embeddings.Embedder
}

type embedderModel interface {
	CreateEmbedding(ctx context.Context, texts []string) ([][]float32, error)
}

func NewActivityEmbedder(model embedderModel) *ActivityEmbedder {
	embedder, _ := embeddings.NewEmbedder(model)
	return &ActivityEmbedder{
		embedder: embedder,
	}
}

func (e *ActivityEmbedder) EmbedActivity(ctx context.Context, act types.Activity, summary *types.ActivitySummary) ([]float32, error) {
	sourceUIDs := act.SourceUIDs()
	sourceUIDsStr := make([]string, len(sourceUIDs))
	for i, sourceUID := range sourceUIDs {
		sourceUIDsStr[i] = sourceUID.String()
	}
	sourceStr := strings.Join(sourceUIDsStr, ", ")

	out, err := e.embedder.EmbedQuery(ctx, fmt.Sprintf("Title: %s\nSources: %s\nSummary: %s", act.Title(), sourceStr, summary.ShortSummary))
	if err != nil {
		return nil, fmt.Errorf("embed activity: %w", err)
	}

	return out, nil
}

func (e *ActivityEmbedder) EmbedActivityQuery(ctx context.Context, query string) ([]float32, error) {
	out, err := e.embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed activity query: %w", err)
	}

	return out, nil
}
