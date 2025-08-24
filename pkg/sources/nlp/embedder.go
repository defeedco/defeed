package nlp

import (
	"context"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/tmc/langchaingo/embeddings"
)

type ActivityEmbedder struct {
	embedder embeddings.Embedder
	// TODO: Implement persistent in the future
	// Note: Persistent cache should be cached per input text and model parameters (e.g. model ID, temperature,...)
	cache map[string][]float32
}

func NewEmbedder(client embeddings.EmbedderClient) *ActivityEmbedder {
	embedder, _ := embeddings.NewEmbedder(client)
	return &ActivityEmbedder{
		embedder: embedder,
		cache:    make(map[string][]float32),
	}
}

func (e *ActivityEmbedder) Embed(ctx context.Context, summary *types.ActivitySummary) ([]float32, error) {
	if out, ok := e.cache[summary.FullSummary]; ok {
		return out, nil
	}

	out, err := e.embedder.EmbedQuery(ctx, summary.FullSummary)
	if err != nil {
		return nil, err
	}

	e.cache[summary.FullSummary] = out
	return out, nil
}
