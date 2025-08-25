package hackernews

import (
	"context"
	"fmt"
	types2 "github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/types"

	"github.com/rs/zerolog"
)

// PostsFetcher implements preset search functionality for HackerNews
type PostsFetcher struct {
	Logger *zerolog.Logger
}

func NewPostsFetcher(logger *zerolog.Logger) *PostsFetcher {
	return &PostsFetcher{
		Logger: logger,
	}
}

func (f *PostsFetcher) SourceType() string {
	return TypeHackerNewsPosts
}

var feedSources = []types.Source{
	&SourcePosts{
		FeedName: "new",
	},
	&SourcePosts{
		FeedName: "top",
	},
	&SourcePosts{
		FeedName: "best",
	},
}

func (f *PostsFetcher) FindByID(ctx context.Context, id types2.TypedUID) (types.Source, error) {
	for _, source := range feedSources {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *PostsFetcher) Search(_ context.Context, _ string) ([]types.Source, error) {
	// Ignore the query, since the set of all available sources is small
	return feedSources, nil
}
