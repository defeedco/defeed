package lobsters

import (
	"context"
	"fmt"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/types"

	"github.com/rs/zerolog"
)

// TagFetcher implements preset search functionality for Lobsters tags
type TagFetcher struct {
	Logger *zerolog.Logger
}

func NewTagFetcher(logger *zerolog.Logger) *TagFetcher {
	return &TagFetcher{
		Logger: logger,
	}
}

func (f *TagFetcher) SourceType() string {
	return TypeLobstersTag
}

var defaultInstanceURL = "https://lobste.rs"
var tagSources = []types.Source{
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "programming",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "javascript",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "rust",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "web",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "security",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "linux",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "opensource",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "distributed",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "crypto",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "containers",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "testing",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "performance",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "algorithms",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "networking",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "mobile",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "devops",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "databases",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "ai",
	},
}

func (f *TagFetcher) FindByID(ctx context.Context, id lib.TypedUID) (types.Source, error) {
	for _, source := range tagSources {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *TagFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	if query == "" {
		return tagSources, nil
	}

	var matchingSources []types.Source
	for _, source := range tagSources {
		if types.IsFuzzyMatch(source, query) {
			matchingSources = append(matchingSources, source)
		}
	}

	// Custom tag (that's not necessarily valid) if no existing ones are found
	// TODO: Handle this better
	if query != "" && len(matchingSources) == 0 {
		source := &SourceTag{
			InstanceURL: defaultInstanceURL,
			Tag:         query,
		}
		matchingSources = append(matchingSources, source)
	}

	return matchingSources, nil
}
