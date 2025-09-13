package lobsters

import (
	"context"
	"fmt"

	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources/types"

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
		InstanceURL:    defaultInstanceURL,
		Tag:            "programming",
		TagDescription: "Use when every tag or no specific tag applies",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "javascript",
		TagDescription: "Javascript programming",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "rust",
		TagDescription: "Rust programming",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "web",
		TagDescription: "Web development and news",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "security",
		TagDescription: "Netsec, appsec, and infosec",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "linux",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "opensource",
		TagDescription: "Open source software and projects",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "distributed",
		TagDescription: "Distributed systems",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "crypto",
		TagDescription: "Cryptocurrency and blockchain",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "containers",
		TagDescription: "Container technologies and orchestration",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "testing",
		TagDescription: "Software testing",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "performance",
		TagDescription: "Performance and optimization",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "algorithms",
		TagDescription: "Algorithm design and analysis",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "networking",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "mobile",
		TagDescription: "Mobile app/web development",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "devops",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "databases",
		TagDescription: "Databases (SQL, NoSQL)",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "ai",
		TagDescription: "Developing artificial intelligence, machine learning.",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "science",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "compsci",
		TagDescription: "Other computer science/programming",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "vibecoding",
		TagDescription: "Using AI/LLM, coding tools.",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "python",
		TagDescription: "Python programming",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "go",
		TagDescription: "Golang programming",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "cloud",
		TagDescription: "Cloud computing and services",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "kubernetes",
		TagDescription: "Kubernetes container orchestration",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "microservices",
		TagDescription: "Microservices architecture",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "api",
		TagDescription: "API development/implementation",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "scaling",
		TagDescription: "Scaling and architecture",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "virtualization",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "wasm",
		TagDescription: "WebAssembly",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "compilers",
		TagDescription: "Compiler design",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "formalmethods",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "plt",
		TagDescription: "Programming language theory, types, design",
	},
	&SourceTag{
		InstanceURL:    defaultInstanceURL,
		Tag:            "cogsci",
		TagDescription: "Cognitive Science",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "cryptography",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "hardware",
	},
	&SourceTag{
		InstanceURL: defaultInstanceURL,
		Tag:         "math",
	},
}

func (f *TagFetcher) FindByID(ctx context.Context, id activitytypes.TypedUID, config *types.ProviderConfig) (types.Source, error) {
	for _, source := range tagSources {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *TagFetcher) Search(ctx context.Context, query string, config *types.ProviderConfig) ([]types.Source, error) {
	// TODO(sources): Support searching custom tags
	// Ignore the query, since the set of all available sources is small
	return tagSources, nil
}
