package rss

import (
	"context"
	_ "embed"
	"fmt"
	types2 "github.com/glanceapp/glance/pkg/sources/activities/types"

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
	OpmlSources []types.Source
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

func (f *FeedFetcher) FindByID(ctx context.Context, id types2.TypedUID) (types.Source, error) {
	for _, source := range f.OpmlSources {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *FeedFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	// Ignore the query, since the set of all available sources is small
	return f.OpmlSources, nil
}

func loadOPMLSources(logger *zerolog.Logger) ([]types.Source, error) {
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

func opmlToRSSSources(opml *lib.OPML) ([]types.Source, error) {
	var result []types.Source

	for _, category := range opml.Body.Outlines {
		for _, outline := range category.Outlines {
			if outline.Type != "rss" {
				return nil, fmt.Errorf("invalid outline type: %s", outline.Type)
			}

			if outline.XMLUrl == "" {
				return nil, fmt.Errorf("outline missing url: %s", outline.Text)
			}

			result = append(result, &SourceFeed{
				Title:   outline.Title,
				FeedURL: outline.XMLUrl,
			})
		}
	}

	return result, nil
}
