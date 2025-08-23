package rss

import (
	"context"
	"strings"

	"github.com/glanceapp/glance/pkg/sources/fetcher"
	"github.com/rs/zerolog"
)

// FeedFetcher implements preset search functionality for RSS feeds
type FeedFetcher struct {
	OpmlSources []fetcher.Source
	Logger      *zerolog.Logger
}

func NewFeedFetcher(opmlSources []fetcher.Source, logger *zerolog.Logger) *FeedFetcher {
	return &FeedFetcher{
		OpmlSources: opmlSources,
		Logger:      logger,
	}
}

func (f *FeedFetcher) Search(ctx context.Context, query string) ([]fetcher.Source, error) {
	if query == "" {
		// Convert to fetcher.Source interface
		var fetcherSources []fetcher.Source
		for _, s := range f.OpmlSources {
			fetcherSources = append(fetcherSources, s)
		}
		return fetcherSources, nil
	}

	query = strings.ToLower(query)
	var matchingSources []fetcher.Source

	for _, source := range f.OpmlSources {
		rssSource, ok := source.(*SourceFeed)
		if !ok {
			continue
		}

		title := strings.ToLower(rssSource.Title)
		url := strings.ToLower(rssSource.FeedURL)

		if strings.Contains(title, query) || strings.Contains(url, query) {
			matchingSources = append(matchingSources, source)
		}
	}

	f.Logger.Debug().
		Str("query", query).
		Int("total_opml", len(f.OpmlSources)).
		Int("matches", len(matchingSources)).
		Msg("RSS fetcher searched OPML sources")

	return matchingSources, nil
}
