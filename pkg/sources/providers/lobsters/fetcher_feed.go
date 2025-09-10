package lobsters

import (
	"context"
	"fmt"

	types2 "github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/types"

	"github.com/rs/zerolog"
)

// FeedFetcher implements preset search functionality for Lobsters
type FeedFetcher struct {
	Logger *zerolog.Logger
}

func NewFeedFetcher(logger *zerolog.Logger) *FeedFetcher {
	return &FeedFetcher{
		Logger: logger,
	}
}

func (f *FeedFetcher) SourceType() string {
	return TypeLobstersFeed
}

var feedSources = []types.Source{
	&SourceFeed{
		InstanceURL: defaultInstanceURL,
		FeedName:    "hottest",
	},
	&SourceFeed{
		InstanceURL: defaultInstanceURL,
		FeedName:    "newest",
	},
}

func (f *FeedFetcher) FindByID(ctx context.Context, id types2.TypedUID, config *types.ProviderConfig) (types.Source, error) {
	for _, source := range feedSources {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *FeedFetcher) Search(_ context.Context, _ string, config *types.ProviderConfig) ([]types.Source, error) {
	// Ignore the query, since the set of all available sources is small
	return feedSources, nil
}
