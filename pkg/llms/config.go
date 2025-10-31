package llms

type Config struct {
	// Embedding
	EmbeddingProvider string `env:"LLM_EMBEDDING_PROVIDER,default=openai"`
	EmbeddingModel    string `env:"LLM_EMBEDDING_MODEL,default=text-embedding-3-large"`

	// Completion
	CompletionProvider string `env:"LLM_COMPLETION_PROVIDER,default=openai"`
	CompletionModel    string `env:"LLM_COMPLETION_MODEL,default=gpt-5-nano-2025-08-07"`

	// Provider specific configurations
	OllamaBaseURL     string `env:"OLLAMA_BASE_URL,default=http://host.docker.internal:11434"` // replace with localhost if running outside docker
	OllamaContextSize int    `env:"OLLAMA_CONTEXT_SIZE,default=32768"`                         // context window size in tokens
}
