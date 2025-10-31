package llms

import (
	"fmt"
	"net/http"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/rs/zerolog"
	"github.com/tmc/langchaingo/llms/openai"
)

func NewCompletionModel(config *Config, logger *zerolog.Logger) (completionModel, error) {
	switch config.CompletionProvider {
	case "openai":
		usageTracker := lib.NewUsageTracker(logger)
		limiter := lib.NewOpenAILimiterWithTracker(logger, usageTracker)
		openaiModel, err := openai.New(
			openai.WithModel(config.CompletionModel),
			openai.WithHTTPClient(limiter),
		)
		if err != nil {
			return nil, fmt.Errorf("create OpenAI model: %w", err)
		}
		return openaiModel, nil
	case "ollama":
		return NewOllamaModel(config.OllamaBaseURL, config.CompletionModel, http.DefaultClient, config.OllamaContextSize), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.CompletionProvider)
	}
}
