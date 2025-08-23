package lobsters

import (
	"context"
	"strings"

	"github.com/glanceapp/glance/pkg/sources/fetcher"
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

var popularLobstersTags = []struct {
	tag         string
	description string
}{
	{"programming", "Programming discussions and articles"},
	{"javascript", "JavaScript programming"},
	{"python", "Python programming"},
	{"go", "Go programming language"},
	{"rust", "Rust programming language"},
	{"web", "Web development"},
	{"security", "Security and cybersecurity topics"},
	{"linux", "Linux operating system"},
	{"opensource", "Open source projects"},
	{"devops", "DevOps and infrastructure"},
	{"databases", "Database technologies"},
	{"ai", "Artificial intelligence"},
	{"networking", "Computer networking"},
	{"mobile", "Mobile development"},
	{"performance", "Performance optimization"},
	{"algorithms", "Algorithms and data structures"},
	{"distributed", "Distributed systems"},
	{"crypto", "Cryptography"},
	{"containers", "Containers and Docker"},
	{"testing", "Software testing"},
}

func (f *TagFetcher) Search(ctx context.Context, query string) ([]fetcher.Source, error) {
	query = strings.ToLower(query)
	var matchingSources []fetcher.Source

	for _, tag := range popularLobstersTags {
		tagName := strings.ToLower(tag.tag)
		description := strings.ToLower(tag.description)

		if query == "" || strings.Contains(tagName, query) || strings.Contains(description, query) {
			source := &SourceTag{
				InstanceURL: "https://lobste.rs",
				Tag:         tag.tag,
			}
			matchingSources = append(matchingSources, source)
		}
	}

	// Custom tag (that's not necessarily valid) if no existing ones are found
	// TODO: Handle this better
	if query == "" && len(matchingSources) == 0 {
		source := &SourceTag{
			InstanceURL: "https://lobste.rs",
			Tag:         query,
		}
		matchingSources = append(matchingSources, source)
	}

	f.Logger.Debug().
		Str("query", query).
		Int("matches", len(matchingSources)).
		Msg("Lobsters Tag fetcher found tags")

	return matchingSources, nil
}
