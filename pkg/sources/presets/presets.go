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
	"golang.org/x/sync/errgroup"
)

// Registry manages available source configurations through fetchers.
type Registry struct {
	fetchers []fetcher.Fetcher
	logger   *zerolog.Logger
}

func NewRegistry(logger *zerolog.Logger) *Registry {
	return &Registry{
		fetchers: make([]fetcher.Fetcher, 0),
		logger:   logger,
	}
}

// Initialize sets up the fetchers for each source type
func (r *Registry) Initialize() error {
	rssFetcher, err := rss.NewFeedFetcher(r.logger)
	if err != nil {
		return fmt.Errorf("initialize RSS fetcher: %w", err)
	}
	r.fetchers = append(r.fetchers, rssFetcher)
	r.fetchers = append(r.fetchers, github.NewIssuesFetcher(r.logger))
	r.fetchers = append(r.fetchers, github.NewReleasesFetcher(r.logger))
	r.fetchers = append(r.fetchers, reddit.NewSubredditFetcher(r.logger))
	r.fetchers = append(r.fetchers, hackernews.NewPostsFetcher(r.logger))
	r.fetchers = append(r.fetchers, lobsters.NewFeedFetcher(r.logger))
	r.fetchers = append(r.fetchers, mastodon.NewAccountFetcher(r.logger))
	r.fetchers = append(r.fetchers, mastodon.NewTagFetcher(r.logger))

	r.logger.Info().
		Int("count", len(r.fetchers)).
		Msg("initialized source fetchers")

	return nil
}

// Search searches for sources from available fetchers
func (r *Registry) Search(ctx context.Context, query string) ([]sources.Source, error) {
	g, gctx := errgroup.WithContext(ctx)

	g.SetLimit(len(r.fetchers))

	results := make([]fetcher.Source, len(r.fetchers))
	for _, f := range r.fetchers {
		g.Go(func() error {
			res, err := f.Search(gctx, query)
			if err != nil {
				return fmt.Errorf("fetcher search: %w", err)
			}
			results = append(results, res...)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("search sources: %w", err)
	}

	r.logger.Debug().
		Str("query", query).
		Int("count", len(results)).
		Msg("searched sources")

	// TODO: rerank results with fuzzy search
	return adaptSources(results), nil
}

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
