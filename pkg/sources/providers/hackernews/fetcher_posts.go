package hackernews

import (
	"context"
	"github.com/glanceapp/glance/pkg/sources/types"
	"strings"

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

var hackerNewsFeeds = []struct {
	name        string
	description string
}{
	{"new", "Latest posts from Hacker News"},
	{"top", "Top posts from Hacker News"},
	{"best", "Best posts from Hacker News"},
}

func (f *PostsFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	query = strings.ToLower(query)
	var matchingSources []types.Source

	for _, feed := range hackerNewsFeeds {
		feedName := strings.ToLower(feed.name)
		description := strings.ToLower(feed.description)

		if query == "" || strings.Contains(feedName, query) || strings.Contains(description, query) {
			source := &SourcePosts{
				FeedName: feed.name,
			}
			matchingSources = append(matchingSources, source)
		}
	}

	f.Logger.Debug().
		Str("query", query).
		Int("matches", len(matchingSources)).
		Msg("HackerNews fetcher found feeds")

	return matchingSources, nil
}
