package lobsters

import (
	"context"
	"github.com/glanceapp/glance/pkg/sources/types"
	"strings"

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

var lobstersFeeds = []struct {
	feedName    string
	description string
}{
	{"hottest", "Hottest posts from Lobsters"},
	{"newest", "Newest posts from Lobsters"},
}

func (f *FeedFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	query = strings.ToLower(query)
	var matchingSources []types.Source

	for _, feed := range lobstersFeeds {
		feedName := strings.ToLower(feed.feedName)
		description := strings.ToLower(feed.description)

		if query == "" || strings.Contains(feedName, query) || strings.Contains(description, query) {
			source := &SourceFeed{
				InstanceURL: "https://lobste.rs",
				FeedName:    feed.feedName,
			}
			matchingSources = append(matchingSources, source)
		}
	}

	f.Logger.Debug().
		Str("query", query).
		Int("matches", len(matchingSources)).
		Msg("Lobsters fetcher found feeds")

	return matchingSources, nil
}
