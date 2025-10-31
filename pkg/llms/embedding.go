package llms

import (
	"fmt"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/rs/zerolog"
	"github.com/tmc/langchaingo/llms/openai"
)

func NewEmbeddingModel(config *Config, logger *zerolog.Logger) (embedderModel, error) {
	switch config.EmbeddingProvider {
	case "openai":
		usageTracker := lib.NewUsageTracker(logger)
		limiter := lib.NewOpenAILimiterWithTracker(logger, usageTracker)
		embeddingModel, err := openai.New(
			openai.WithEmbeddingModel("text-embedding-3-large"),
			openai.WithHTTPClient(limiter),
		)
		if err != nil {
			return nil, fmt.Errorf("create openai embedding model: %w", err)
		}
		return embeddingModel, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.EmbeddingProvider)
	}
}
