package mastodon

import (
	"context"
	"strings"

	"github.com/glanceapp/glance/pkg/sources/fetcher"
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

var popularMastodonTags = []struct {
	tag         string
	description string
}{
	{"programming", "Programming discussions"},
	{"technology", "Technology news and updates"},
	{"opensource", "Open source projects and discussions"},
	{"privacy", "Privacy-focused discussions"},
	{"security", "Cybersecurity topics"},
	{"linux", "Linux operating system"},
	{"javascript", "JavaScript programming"},
	{"python", "Python programming"},
	{"golang", "Go programming language"},
	{"rust", "Rust programming language"},
}

func (f *TagFetcher) Search(ctx context.Context, query string) ([]fetcher.Source, error) {
	query = strings.ToLower(query)
	var matchingSources []fetcher.Source

	for _, tag := range popularMastodonTags {
		tagName := strings.ToLower(tag.tag)
		description := strings.ToLower(tag.description)

		if query == "" || strings.Contains(tagName, query) || strings.Contains(description, query) {
			source := &SourceTag{
				InstanceURL: "https://mastodon.social",
				Tag:         tag.tag,
			}
			matchingSources = append(matchingSources, source)
		}
	}

	// Also add empty template for user customization
	if query == "" || len(matchingSources) == 0 {
		source := &SourceTag{
			InstanceURL: "https://mastodon.social",
			Tag:         "",
		}
		matchingSources = append(matchingSources, source)
	}

	f.Logger.Debug().
		Str("query", query).
		Int("matches", len(matchingSources)).
		Msg("Mastodon Tag fetcher found tags")

	return matchingSources, nil
}
