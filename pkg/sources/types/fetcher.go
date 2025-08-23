package types

import "context"

// Fetcher interface allows source types to provide preset/search functionality.
// Each source type can implement this to either:
// - Return static presets filtered by fuzzy search
// - Query external APIs to find matching sources
type Fetcher interface {
	Search(ctx context.Context, query string) ([]Source, error)
}
