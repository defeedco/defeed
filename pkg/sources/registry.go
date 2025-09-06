package sources

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sort"

	activitytypes "github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/glanceapp/glance/pkg/sources/types"

	"strings"

	"github.com/glanceapp/glance/pkg/sources/providers/github"
	"github.com/glanceapp/glance/pkg/sources/providers/hackernews"
	"github.com/glanceapp/glance/pkg/sources/providers/lobsters"
	"github.com/glanceapp/glance/pkg/sources/providers/mastodon"
	"github.com/glanceapp/glance/pkg/sources/providers/reddit"
	"github.com/glanceapp/glance/pkg/sources/providers/rss"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// Registry manages available source configurations through fetchers.
type Registry struct {
	fetchers []types.Fetcher
	logger   *zerolog.Logger
}

func NewRegistry(logger *zerolog.Logger) *Registry {
	return &Registry{
		fetchers: make([]types.Fetcher, 0),
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
	r.fetchers = append(r.fetchers, github.NewTopicFetcher(r.logger))
	r.fetchers = append(r.fetchers, reddit.NewSubredditFetcher(r.logger))
	r.fetchers = append(r.fetchers, hackernews.NewPostsFetcher(r.logger))
	r.fetchers = append(r.fetchers, lobsters.NewFeedFetcher(r.logger))
	r.fetchers = append(r.fetchers, lobsters.NewTagFetcher(r.logger))
	r.fetchers = append(r.fetchers, mastodon.NewAccountFetcher(r.logger))
	r.fetchers = append(r.fetchers, mastodon.NewTagFetcher(r.logger))

	r.logger.Info().
		Int("count", len(r.fetchers)).
		Msg("initialized source fetchers")

	return nil
}

func (r *Registry) FindByUID(ctx context.Context, uid activitytypes.TypedUID) (types.Source, error) {
	var fetcher types.Fetcher
	for _, f := range r.fetchers {
		if f.SourceType() == uid.Type() {
			fetcher = f
			break
		}
	}
	if fetcher == nil {
		return nil, errors.New("source not found")
	}

	source, err := fetcher.FindByID(ctx, uid)
	if err != nil {
		return nil, err
	}

	return source, nil
}

// Search searches for sources from available fetchers
func (r *Registry) Search(ctx context.Context, query string) ([]types.Source, error) {
	g, gctx := errgroup.WithContext(ctx)

	g.SetLimit(len(r.fetchers))

	results := make([]types.Source, 0)
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

	var filtered []types.Source
	if query == "" {
		filtered = alphabeticalSort(results)
	} else {
		filtered = fuzzyReRank(results, query)
	}

	return filtered, nil
}

// sourceWithSearchText holds a source and its searchable text for fuzzy matching
type sourceWithSearchText struct {
	source     types.Source
	searchText string
}

// fuzzyReRank reranks sources using fuzzy search scoring
func fuzzyReRank(input []types.Source, query string) []types.Source {
	if len(input) == 0 || query == "" {
		return input
	}

	query = strings.TrimSpace(query)

	sourcesWithText := make([]sourceWithSearchText, len(input))
	searchTexts := make([]string, len(input))

	for i, source := range input {
		searchText := buildSearchText(source)
		sourcesWithText[i] = sourceWithSearchText{
			source:     source,
			searchText: searchText,
		}
		searchTexts[i] = searchText
	}

	ranks := fuzzy.RankFindNormalizedFold(query, searchTexts)

	result := make([]types.Source, len(ranks))
	for i, rank := range ranks {
		result[i] = sourcesWithText[rank.OriginalIndex].source
	}

	return result
}

// alphabeticalSort sorts sources alphabetically by name (case-insensitive)
func alphabeticalSort(input []types.Source) []types.Source {
	sorted := make([]types.Source, len(input))
	copy(sorted, input)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i].Name()) < strings.ToLower(sorted[j].Name())
	})
	return sorted
}

// buildSearchText creates a searchable text string from a source's key fields
func buildSearchText(source types.Source) string {
	parts := []string{
		source.Name(),
		source.Description(),
	}

	var nonEmptyParts []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			nonEmptyParts = append(nonEmptyParts, part)
		}
	}

	return strings.Join(nonEmptyParts, " ")
}
