package nlp

import (
	"context"
	"fmt"
	"github.com/tmc/langchaingo/embeddings"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
)

type ActivityEmbedder struct {
	embedder embeddings.Embedder
}

type embedderModel interface {
	CreateEmbedding(ctx context.Context, texts []string) ([][]float32, error)
}

func NewEmbedder(model embedderModel) *ActivityEmbedder {
	embedder, _ := embeddings.NewEmbedder(model)
	return &ActivityEmbedder{
		embedder: embedder,
	}
}

func (e *ActivityEmbedder) Embed(ctx context.Context, summary *types.ActivitySummary) ([]float32, error) {
	out, err := e.embedder.EmbedQuery(ctx, summary.FullSummary)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	return out, nil
}
