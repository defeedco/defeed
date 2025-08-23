package fetcher

import "context"

// Source represents a minimal interface for sources to avoid import cycles.
// This allows fetchers to return sources without importing the full sources package.
type Source interface {
	UID() string
	Name() string
	Type() string
}

// Fetcher interface allows source types to provide preset/search functionality.
// Each source type can implement this to either:
// - Return static presets filtered by fuzzy search
// - Query external APIs to find matching sources
type Fetcher interface {
	Search(ctx context.Context, query string) ([]Source, error)
}
