package nlp

import (
	"context"
	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/tmc/langchaingo/embeddings"
)

type ActivityEmbedder struct {
	embedder embeddings.Embedder
}

func NewEmbedder(client embeddings.EmbedderClient) *ActivityEmbedder {
	embedder, _ := embeddings.NewEmbedder(client)
	return &ActivityEmbedder{embedder: embedder}
}

func (e *ActivityEmbedder) Embed(ctx context.Context, summary *types.ActivitySummary) ([]float32, error) {
	out, err := e.embedder.EmbedQuery(ctx, summary.FullSummary)
	if err != nil {
		return nil, err
	}
	return out, nil
}
