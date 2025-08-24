package rss

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/types"

	"github.com/rs/zerolog"
)

// Source: https://raw.githubusercontent.com/tuan3w/awesome-tech-rss/refs/heads/main/feeds.opml
//
//go:embed awesome-tech-rss.opml
var awesomeTechRSS string

// FeedFetcher implements preset search functionality for RSS feeds
type FeedFetcher struct {
	OpmlSources []*SourceFeed
	Logger      *zerolog.Logger
}

func NewFeedFetcher(logger *zerolog.Logger) (*FeedFetcher, error) {
	opmlSources, err := loadOPMLSources(logger)
	if err != nil {
		return nil, fmt.Errorf("load OPML sources: %w", err)
	}
	return &FeedFetcher{
		OpmlSources: opmlSources,
		Logger:      logger,
	}, nil
}

func (f *FeedFetcher) SourceType() string {
	return TypeRSSFeed
}

func (f *FeedFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	if query == "" {
		// Convert to fetcher.Source interface
		var fetcherSources []types.Source
		for _, s := range f.OpmlSources {
			fetcherSources = append(fetcherSources, s)
		}
		return fetcherSources, nil
	}

	query = strings.ToLower(query)
	var matchingSources []types.Source

	for _, rssSource := range f.OpmlSources {
		title := strings.ToLower(rssSource.Title)
		url := strings.ToLower(rssSource.FeedURL)

		if strings.Contains(title, query) || strings.Contains(url, query) {
			matchingSources = append(matchingSources, rssSource)
		}
	}

	f.Logger.Debug().
		Str("query", query).
		Int("total_opml", len(f.OpmlSources)).
		Int("matches", len(matchingSources)).
		Msg("RSS fetcher searched OPML sources")

	return matchingSources, nil
}

func loadOPMLSources(logger *zerolog.Logger) ([]*SourceFeed, error) {
	opml, err := lib.ParseOPML(awesomeTechRSS)
	if err != nil {
		return nil, fmt.Errorf("parse OPML: %w", err)
	}

	opmlSources, err := opmlToRSSSources(opml)
	if err != nil {
		return nil, fmt.Errorf("convert OPML to RSS sources: %w", err)
	}

	logger.Info().
		Int("count", len(opmlSources)).
		Msg("loaded OPML RSS sources")

	return opmlSources, nil
}

func opmlToRSSSources(opml *lib.OPML) ([]*SourceFeed, error) {
	var result []*SourceFeed

	for _, category := range opml.Body.Outlines {
		for _, outline := range category.Outlines {
			if outline.Type != "rss" {
				return nil, fmt.Errorf("invalid outline type: %s", outline.Type)
			}

			if outline.XMLUrl == "" {
				return nil, fmt.Errorf("outline missing url: %s", outline.Text)
			}

			rssSource := &SourceFeed{
				Title:   outline.Title,
				FeedURL: outline.XMLUrl,
			}

			result = append(result, rssSource)
		}
	}

	return result, nil
}
