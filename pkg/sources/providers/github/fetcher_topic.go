package github

import (
    "context"
    "fmt"
    types2 "github.com/glanceapp/glance/pkg/sources/activities/types"
    "strings"

    "github.com/glanceapp/glance/pkg/sources/types"
    "github.com/rs/zerolog"
)

// TopicFetcher provides preset/search functionality for GitHub Topic sources
type TopicFetcher struct {
    Logger *zerolog.Logger
}

func NewTopicFetcher(logger *zerolog.Logger) *TopicFetcher {
    return &TopicFetcher{Logger: logger}
}

func (f *TopicFetcher) SourceType() string {
    return TypeGithubTopic
}

func (f *TopicFetcher) FindByID(ctx context.Context, id types2.TypedUID) (types.Source, error) {
    // ID format: githubtopic:topic[:mode]
    parts := strings.Split(id.String(), ":")
    if len(parts) < 2 {
        return nil, fmt.Errorf("invalid github topic uid: %s", id.String())
    }
    var topic string
    var mode string
    if len(parts) >= 2 {
        topic = parts[1]
    }
    if len(parts) >= 3 {
        mode = parts[2]
    }

    return &SourceTopic{
        Topic: topic,
        Mode:  mode,
    }, nil
}

func (f *TopicFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
    trimmed := strings.TrimSpace(query)
    if trimmed == "" {
        // Cannot enumerate all topics; return no presets
        return []types.Source{}, nil
    }

    // Propose two presets: trending and new for the given topic
    trending := &SourceTopic{Topic: trimmed, Mode: "trending"}
    newest := &SourceTopic{Topic: trimmed, Mode: "new"}

    f.Logger.Debug().
        Str("topic", trimmed).
        Msg("GitHub Topic fetcher created presets")

    return []types.Source{trending, newest}, nil
}

