package presets

import (
	_ "embed"
	"fmt"

	"github.com/glanceapp/glance/pkg/sources/providers/rss"
	"github.com/rs/zerolog"

	"github.com/glanceapp/glance/pkg/sources"
)

// Source: https://raw.githubusercontent.com/tuan3w/awesome-tech-rss/refs/heads/main/feeds.opml
//
//go:embed awesome-tech-rss-feed.opml
var awesomeTechRSSFeedOPMLRaw string

// Registry manages available source configurations.
type Registry struct {
	presets map[string]sources.Source
	logger  *zerolog.Logger
}

func NewRegistry(logger *zerolog.Logger) *Registry {
	return &Registry{
		presets: make(map[string]sources.Source),
		logger:  logger,
	}
}

// Initialize parses the embedded OPML file and populates the registry with sources
func (r *Registry) Initialize() error {

	awesomeTechRSSFeedOPML, err := ParseOPML(awesomeTechRSSFeedOPMLRaw)
	if err != nil {
		return fmt.Errorf("parse awesome-tech-rss-feed.opml: %w", err)
	}

	awesomeTechRSSSources, err := opmlToRSSSources(awesomeTechRSSFeedOPML)
	if err != nil {
		return fmt.Errorf("convert OPML to RSS sources: %w", err)
	}

	r.logger.Info().Msgf("found %d RSS presets in awesome-tech-rss-feed.opml", len(awesomeTechRSSSources))

	for _, s := range awesomeTechRSSSources {
		err := r.add(s)
		if err != nil {
			// Ignore any duplicate entries that were present in the file
			r.logger.Error().Err(err).Msgf("failed to add RSS preset: %s", s.UID())
		}
	}

	return nil
}

func (r *Registry) add(s sources.Source) error {
	if _, ok := r.presets[s.UID()]; ok {
		return fmt.Errorf("source preset already exists: %s", s.UID())
	}

	r.presets[s.UID()] = s

	return nil
}

func opmlToRSSSources(opml *OPML) ([]sources.Source, error) {
	var result []sources.Source

	for _, category := range opml.Body.Outlines {
		for _, outline := range category.Outlines {
			if outline.Type != "rss" {
				return nil, fmt.Errorf("invalid outline type: %s", outline.Type)
			}

			if outline.XMLUrl == "" {
				return nil, fmt.Errorf("outline missing url: %s", outline.Text)
			}

			source, err := sources.NewSource(rss.TypeRSSFeed)
			if err != nil {
				return nil, fmt.Errorf("new rss source: %w", err)
			}

			rssSource, ok := source.(*rss.SourceFeed)
			if !ok {
				return nil, fmt.Errorf("cast source to rss source: %w", err)
			}

			rssSource.FeedURL = outline.XMLUrl
			rssSource.Title = outline.Title

			result = append(result, source)
		}
	}

	return result, nil
}

// Sources returns all preset sources
func (r *Registry) Sources() ([]sources.Source, error) {
	var result = make([]sources.Source, 0, len(r.presets))

	for _, s := range r.presets {
		result = append(result, s)
	}

	return result, nil
}
