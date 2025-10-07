package rss

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources/types"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// Modified from: https://raw.githubusercontent.com/tuan3w/awesome-tech-rss/refs/heads/main/feeds.opml
//
//go:embed feeds.opml
var feedsOPML string

// FeedFetcher implements preset search functionality for RSS feeds
type FeedFetcher struct {
	// Feeds are the most relevant predefined feeds
	Feeds  []types.Source
	Logger *zerolog.Logger
}

func NewFeedFetcher(logger *zerolog.Logger) *FeedFetcher {
	feeds, err := loadOPMLSources(logger)
	if err != nil {
		logger.Error().Err(err).Msg("load OPML sources")
		return nil
	}

	fetchIcons(context.Background(), logger, feeds)

	return &FeedFetcher{
		Feeds:  feeds,
		Logger: logger,
	}
}

func (f *FeedFetcher) SourceType() string {
	return TypeRSSFeed
}

func (f *FeedFetcher) FindByID(ctx context.Context, id activitytypes.TypedUID, config *types.ProviderConfig) (types.Source, error) {
	for _, source := range f.Feeds {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *FeedFetcher) Search(ctx context.Context, query string, config *types.ProviderConfig) ([]types.Source, error) {
	// TODO(sources): Support adding custom feed URL?
	// Ignore the query, since the set of all available sources is small
	return f.Feeds, nil
}

func loadOPMLSources(logger *zerolog.Logger) ([]types.Source, error) {
	opml, err := lib.ParseOPML(feedsOPML)
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

	seen := make(map[string]bool)
	for _, category := range opml.Body.Outlines {
		for _, outline := range category.Outlines {
			if outline.Type != "rss" {
				return nil, fmt.Errorf("invalid outline type: %s", outline.Type)
			}

			if outline.XMLUrl == "" {
				return nil, fmt.Errorf("outline missing url: %s", outline.Text)
			}

			source := &SourceFeed{
				title:       outline.Title,
				FeedURL:     outline.XMLUrl,
				description: outline.Text,
				IconURL:     outline.FaviconUrl,
			}

			if outline.Topics != "" {
				topicStrings := strings.Split(outline.Topics, ",")
				var topicTags []types.TopicTag
				for _, topicStr := range topicStrings {
					tag, ok := types.WordToTopic(topicStr)
					if !ok {
						return nil, fmt.Errorf("invalid topic: %s", topicStr)
					}

					topicTags = append(topicTags, tag)
				}
				source.topics = topicTags
			}

			if seen[source.UID().String()] {
				continue
			}
			seen[source.UID().String()] = true

			result = append(result, source)
		}
	}

	return result, nil
}

func fetchIcons(ctx context.Context, logger *zerolog.Logger, sources []types.Source) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(len(sources))

	for _, source := range sources {
		g.Go(func() error {
			ctx, cancel := context.WithTimeout(gctx, 10*time.Second)
			defer cancel()

			feedSource := source.(*SourceFeed)
			if err := feedSource.fetchIcon(ctx, logger); err != nil {
				logger.Error().Err(err).
					Str("source", feedSource.UID().String()).
					Msg("fetch icon for feed source")
			}
			return nil
		})
	}

	// Ignore errors
	_ = g.Wait()
}
