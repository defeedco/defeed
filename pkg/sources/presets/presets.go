package presets

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/glanceapp/glance/pkg/sources"
	"github.com/glanceapp/glance/pkg/sources/fetcher"
	"github.com/glanceapp/glance/pkg/sources/providers/github"
	"github.com/glanceapp/glance/pkg/sources/providers/hackernews"
	"github.com/glanceapp/glance/pkg/sources/providers/lobsters"
	"github.com/glanceapp/glance/pkg/sources/providers/mastodon"
	"github.com/glanceapp/glance/pkg/sources/providers/reddit"
	"github.com/glanceapp/glance/pkg/sources/providers/rss"
	"github.com/rs/zerolog"
)

// Source: https://raw.githubusercontent.com/tuan3w/awesome-tech-rss/refs/heads/main/feeds.opml
//
//go:embed awesome-tech-rss-list.opml
var awesomeTechRSSListRaw string

// Adapter function to convert fetcher.Source to sources.Source
func adaptSources(fetcherSources []fetcher.Source) []sources.Source {
	var result []sources.Source
	for _, fs := range fetcherSources {
		if s, ok := fs.(sources.Source); ok {
			result = append(result, s)
		}
	}
	return result
}

// Registry manages available source configurations through fetchers.
type Registry struct {
	fetchers map[string]fetcher.Fetcher
	logger   *zerolog.Logger
}

func NewRegistry(logger *zerolog.Logger) *Registry {
	return &Registry{
		fetchers: make(map[string]fetcher.Fetcher),
		logger:   logger,
	}
}

// Initialize sets up the fetchers for each source type
func (r *Registry) Initialize() error {
	err := r.initializeFetchers()
	if err != nil {
		return fmt.Errorf("initialize fetchers: %w", err)
	}

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

			rssSource := &rss.SourceFeed{
				Title:   outline.Title,
				FeedURL: outline.XMLUrl,
			}

			result = append(result, rssSource)
		}
	}

	return result, nil
}

func (r *Registry) initializeFetchers() error {
	// Initialize RSS fetcher with OPML data
	opmlSources, err := r.loadOPMLSources()
	if err != nil {
		return fmt.Errorf("load OPML sources: %w", err)
	}

	// Convert OPML sources to fetcher.Source interface
	var fetcherSources []fetcher.Source
	for _, s := range opmlSources {
		fetcherSources = append(fetcherSources, s)
	}

	// Register fetchers for each source type
	r.fetchers["github:issues"] = github.NewIssuesFetcher(r.logger)
	r.fetchers["github:releases"] = github.NewReleasesFetcher(r.logger)
	r.fetchers["rss:feed"] = rss.NewFeedFetcher(fetcherSources, r.logger)
	r.fetchers["reddit:subreddit"] = reddit.NewSubredditFetcher(r.logger)
	r.fetchers["hackernews:posts"] = hackernews.NewPostsFetcher(r.logger)
	r.fetchers["lobsters:feed"] = lobsters.NewFeedFetcher(r.logger)
	r.fetchers["mastodon:account"] = mastodon.NewAccountFetcher(r.logger)
	r.fetchers["mastodon:tag"] = mastodon.NewTagFetcher(r.logger)

	r.logger.Info().
		Int("fetchers", len(r.fetchers)).
		Msg("initialized source fetchers")

	return nil
}

func (r *Registry) loadOPMLSources() ([]sources.Source, error) {
	opml, err := ParseOPML(awesomeTechRSSListRaw)
	if err != nil {
		return nil, fmt.Errorf("parse OPML: %w", err)
	}

	opmlSources, err := opmlToRSSSources(opml)
	if err != nil {
		return nil, fmt.Errorf("convert OPML to RSS sources: %w", err)
	}

	r.logger.Info().
		Int("count", len(opmlSources)).
		Msg("loaded OPML RSS sources")

	return opmlSources, nil
}

// SearchSources searches for sources using the appropriate fetcher based on source type
func (r *Registry) SearchSources(ctx context.Context, sourceType, query string) ([]sources.Source, error) {
	fetcherImpl, exists := r.fetchers[sourceType]
	if !exists {
		return nil, fmt.Errorf("no fetcher available for source type: %s", sourceType)
	}

	fetcherSources, err := fetcherImpl.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search failed for type %s: %w", sourceType, err)
	}

	sources := adaptSources(fetcherSources)

	r.logger.Debug().
		Str("source_type", sourceType).
		Str("query", query).
		Int("results", len(sources)).
		Msg("searched sources")

	return sources, nil
}

// GetAvailableSourceTypes returns the source types that have fetchers available
func (r *Registry) GetAvailableSourceTypes() []string {
	var types []string
	for sourceType := range r.fetchers {
		types = append(types, sourceType)
	}
	return types
}

// GetAllPresets returns presets from all fetchers (for backward compatibility)
func (r *Registry) GetAllPresets(ctx context.Context) (map[string][]sources.Source, error) {
	result := make(map[string][]sources.Source)

	for sourceType, fetcherImpl := range r.fetchers {
		fetcherSources, err := fetcherImpl.Search(ctx, "")
		if err != nil {
			r.logger.Warn().
				Err(err).
				Str("source_type", sourceType).
				Msg("failed to get presets for source type")
			continue
		}
		result[sourceType] = adaptSources(fetcherSources)
	}

	return result, nil
}
