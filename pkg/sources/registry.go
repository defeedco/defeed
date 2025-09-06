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

// sourceWithScore holds a source and its calculated relevance score
type sourceWithScore struct {
	source types.Source
	score  float64
}

// fuzzyReRank reranks sources using improved fuzzy search scoring
func fuzzyReRank(input []types.Source, query string) []types.Source {
	if len(input) == 0 || query == "" {
		return input
	}

	query = strings.TrimSpace(strings.ToLower(query))

	sourcesWithScore := make([]sourceWithScore, len(input))

	for i, source := range input {
		score := calculateRelevanceScore(source, query)
		sourcesWithScore[i] = sourceWithScore{
			source: source,
			score:  score,
		}
	}

	// Sort by score (higher is better)
	sort.Slice(sourcesWithScore, func(i, j int) bool {
		return sourcesWithScore[i].score > sourcesWithScore[j].score
	})

	result := make([]types.Source, len(sourcesWithScore))
	for i, item := range sourcesWithScore {
		result[i] = item.source
	}

	return result
}

// calculateRelevanceScore calculates a relevance score for a source based on the query
func calculateRelevanceScore(source types.Source, query string) float64 {
	score := 0.0

	name := strings.ToLower(strings.TrimSpace(source.Name()))
	description := strings.ToLower(strings.TrimSpace(source.Description()))

	// Exact match in title gets highest score
	if name == query {
		score += 100.0
	} else if strings.Contains(name, query) {
		// Partial match in title gets high score
		// Score based on how much of the title matches
		matchRatio := float64(len(query)) / float64(len(name))
		score += 50.0 * matchRatio
	}

	// Check for fuzzy match in title using Levenshtein distance
	if name != "" {
		distance := fuzzy.LevenshteinDistance(query, name)
		maxLen := max(len(query), len(name))
		if maxLen > 0 {
			similarity := 1.0 - float64(distance)/float64(maxLen)
			if similarity > 0.6 { // Only consider if reasonably similar
				score += 30.0 * similarity
			}
		}
	}

	// Exact match in description gets medium score
	if strings.Contains(description, query) {
		// Score based on position - earlier matches are better
		pos := strings.Index(description, query)
		positionScore := 1.0 - float64(pos)/float64(max(len(description), 1))
		score += 20.0 * positionScore
	}

	// Fuzzy match in description gets lower score
	if description != "" {
		distance := fuzzy.LevenshteinDistance(query, description)
		maxLen := max(len(query), len(description))
		if maxLen > 0 {
			similarity := 1.0 - float64(distance)/float64(maxLen)
			if similarity > 0.7 { // Higher threshold for description
				score += 10.0 * similarity
			}
		}
	}

	return score
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
