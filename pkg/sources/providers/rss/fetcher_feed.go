package rss

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources/types"

	"github.com/rs/zerolog"
)

// Modified from: https://raw.githubusercontent.com/tuan3w/awesome-tech-rss/refs/heads/main/feeds.opml
//
//go:embed feeds.opml
var feedsOPML string

//go:embed faviconmap.json
var faviconMapJSON string

// FeedFetcher implements preset search functionality for RSS feeds
type FeedFetcher struct {
	// Feeds are the most relevant predefined feeds
	Feeds  []types.Source
	Logger *zerolog.Logger
}

func NewFeedFetcher(logger *zerolog.Logger) *FeedFetcher {
	var faviconMap map[string]string
	err := json.Unmarshal([]byte(faviconMapJSON), &faviconMap)
	if err != nil {
		logger.Error().Err(err).Msg("parse favicon map")
		return nil
	}

	feeds, err := loadOPMLSources(logger, faviconMap)
	if err != nil {
		logger.Fatal().Err(err).Msg("load OPML sources")
		return nil
	}

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

func loadOPMLSources(logger *zerolog.Logger, faviconMap map[string]string) ([]types.Source, error) {
	opml, err := lib.ParseOPML(feedsOPML)
	if err != nil {
		return nil, fmt.Errorf("parse OPML: %w", err)
	}

	opmlSources, err := opmlToRSSSources(logger, opml, faviconMap)
	if err != nil {
		return nil, fmt.Errorf("convert OPML to RSS sources: %w", err)
	}

	logger.Info().
		Int("count", len(opmlSources)).
		Msg("loaded OPML RSS sources")

	return opmlSources, nil
}

func opmlToRSSSources(logger *zerolog.Logger, opml *lib.OPML, faviconMap map[string]string) ([]types.Source, error) {
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
				topics:      []types.TopicTag{},
			}

			if source.IconURL == "" {
				hostName, err := lib.StripURLHost(outline.XMLUrl)
				if err != nil {
					logger.Error().Err(err).
						Str("url", outline.XMLUrl).
						Msg("invalid url")
					continue
				}
				if faviconURL, ok := faviconMap[hostName]; ok {
					source.IconURL = faviconURL
				}
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
