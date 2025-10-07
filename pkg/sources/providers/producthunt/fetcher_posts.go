package producthunt

import (
	"context"
	"fmt"

	"github.com/defeedco/defeed/pkg/lib"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/defeedco/defeed/pkg/sources/types"
	"github.com/rs/zerolog"
)

type PostsFetcher struct {
	Logger *zerolog.Logger
}

func NewPostsFetcher(logger *zerolog.Logger) *PostsFetcher {
	return &PostsFetcher{
		Logger: logger,
	}
}

func (f *PostsFetcher) SourceType() string {
	return TypeProductHuntPosts
}

var feedSources = []types.Source{
	&SourcePosts{
		FeedName: "top",
	},
	&SourcePosts{
		FeedName: "new",
	},
}

func (f *PostsFetcher) FindByID(ctx context.Context, id activitytypes.TypedUID, config *types.ProviderConfig) (types.Source, error) {
	for _, source := range feedSources {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *PostsFetcher) Search(_ context.Context, _ string, config *types.ProviderConfig) ([]types.Source, error) {
	return feedSources, nil
}
