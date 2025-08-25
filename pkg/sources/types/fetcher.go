package types

import (
	"context"
	types2 "github.com/glanceapp/glance/pkg/sources/activities/types"
)

// Fetcher interface allows source types to provide preset/search functionality.
// Each source type can implement this to either:
// - Return static presets filtered by fuzzy search
// - Query external APIs to find matching sources
type Fetcher interface {
	SourceType() string
	FindByID(ctx context.Context, id types2.TypedUID) (Source, error)
	// Search can either:
	// - return a full list of available sources when query is empty or when the set of all available sources is small (e.g. Lobsters Feeds)
	// - return a filtered list of sources when query is non-empty or the set of all available sources is large (e.g. GitHub Issues)
	Search(ctx context.Context, query string) ([]Source, error)
}
