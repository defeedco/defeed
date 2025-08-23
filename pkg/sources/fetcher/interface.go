package fetcher

import "context"

// Source represents a minimal interface for sources to avoid import cycles.
// This allows fetchers to return sources without importing the full sources package.
type Source interface {
	// UID is the unique identifier for the source.
	// It should not contain any slashes.
	UID() string
	// Type is the non-parameterized ID (e.g. "reddit:subreddit" vs "reddit:subreddit:top:day")
	Type() string
	// Name is a short human-readable descriptor.
	// Example: "Programming Subreddit"
	Name() string
	// Description provides more context about the specific source parameters.
	// Example: "Top posts from r/programming"
	Description() string
}

// Fetcher interface allows source types to provide preset/search functionality.
// Each source type can implement this to either:
// - Return static presets filtered by fuzzy search
// - Query external APIs to find matching sources
type Fetcher interface {
	Search(ctx context.Context, query string) ([]Source, error)
}
