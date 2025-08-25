package mastodon

import (
	"context"
	"fmt"

	types2 "github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/types"

	"github.com/rs/zerolog"
)

// TagFetcher implements preset search functionality for Mastodon hashtags
type TagFetcher struct {
	Logger *zerolog.Logger
}

func NewTagFetcher(logger *zerolog.Logger) *TagFetcher {
	return &TagFetcher{
		Logger: logger,
	}
}

func (f *TagFetcher) SourceType() string {
	return TypeMastodonTag
}

var popularTagSources = []types.Source{
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "programming",
		TagSummary:  "Programming discussions",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "technology",
		TagSummary:  "Technology news and updates",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "opensource",
		TagSummary:  "Open source projects and discussions",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "privacy",
		TagSummary:  "Privacy-focused discussions",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "security",
		TagSummary:  "Cybersecurity topics",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "linux",
		TagSummary:  "Linux operating system",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "javascript",
		TagSummary:  "JavaScript programming",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "python",
		TagSummary:  "Python programming",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "golang",
		TagSummary:  "Go programming language",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "rust",
		TagSummary:  "Rust programming language",
	},
}

func (f *TagFetcher) FindByID(ctx context.Context, id types2.TypedUID) (types.Source, error) {
	for _, source := range popularTagSources {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *TagFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	// TODO(sources): Support searching custom tags
	// Ignore the query, since the set of all available sources is small
	return popularTagSources, nil
}
