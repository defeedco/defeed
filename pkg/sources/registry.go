package sources

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sort"

	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"

	"github.com/defeedco/defeed/pkg/sources/types"

	"strings"

	"github.com/defeedco/defeed/pkg/sources/providers/github"
	"github.com/defeedco/defeed/pkg/sources/providers/hackernews"
	"github.com/defeedco/defeed/pkg/sources/providers/lobsters"
	"github.com/defeedco/defeed/pkg/sources/providers/mastodon"
	"github.com/defeedco/defeed/pkg/sources/providers/reddit"
	"github.com/defeedco/defeed/pkg/sources/providers/rss"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// Registry manages available source configurations through fetchers.
type Registry struct {
	fetchers     []types.Fetcher
	logger       *zerolog.Logger
	sourceConfig *types.ProviderConfig
}

func NewRegistry(logger *zerolog.Logger, sourceConfig *types.ProviderConfig) *Registry {
	return &Registry{
		fetchers:     make([]types.Fetcher, 0),
		logger:       logger,
		sourceConfig: sourceConfig,
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

	source, err := fetcher.FindByID(ctx, uid, r.sourceConfig)
	if err != nil {
		return nil, err
	}

	return source, nil
}

// SearchRequest configures how sources are searched and ranked.
type SearchRequest struct {
	Query  string
	Topics []types.TopicTag
}

// Search searches for sources from available fetchers
func (r *Registry) Search(ctx context.Context, params SearchRequest) ([]types.Source, error) {
	g, gctx := errgroup.WithContext(ctx)

	g.SetLimit(len(r.fetchers))

	results := make([]types.Source, 0)
	for _, f := range r.fetchers {
		g.Go(func() error {
			res, err := f.Search(gctx, params.Query, r.sourceConfig)
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
		Str("query", params.Query).
		Int("count", len(results)).
		Msg("searched sources")

	if len(params.Topics) > 0 {
		results = filterByTopics(results, params.Topics)
	}

	switch {
	case params.Query != "":
		results = fuzzyReRank(results, params.Query)
	default:
		results = curatedDefaultSort(results)
	}

	return results, nil
}

func filterByTopics(input []types.Source, topics []types.TopicTag) []types.Source {
	result := make([]types.Source, 0)

	lookup := make(map[types.TopicTag]bool)
	for _, topic := range topics {
		lookup[topic] = true
	}

	for _, source := range input {
	topics:
		for _, topic := range source.Topics() {
			if lookup[topic] {
				result = append(result, source)
				break topics
			}
		}
	}

	return result
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

	result := make([]types.Source, 0)
	for _, item := range sourcesWithScore {
		// Exclude less relevant search results
		if item.score > 20 {
			result = append(result, item.source)
		}
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

// curatedDefaultSort provides a non-personalized default ordering prioritizing
// familiar, high-signal sources first so the UI feels less overwhelming.
func curatedDefaultSort(input []types.Source) []types.Source {
	sorted := make([]types.Source, len(input))
	copy(sorted, input)

	baseWeight := func(s types.Source) int {
		switch s.UID().Type() {
		case reddit.TypeRedditSubreddit:
			return 100
		case hackernews.TypeHackerNewsPosts:
			return 95
		case lobsters.TypeLobstersTag, lobsters.TypeLobstersFeed:
			return 90
		case github.TypeGithubIssues, github.TypeGithubReleases:
			return 80
		case rss.TypeRSSFeed:
			return 70
		case mastodon.TypeMastodonAccount, mastodon.TypeMastodonTag:
			return 65
		default:
			return 50
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		wi := baseWeight(sorted[i])
		wj := baseWeight(sorted[j])
		if wi == wj {
			return strings.ToLower(sorted[i].Name()) < strings.ToLower(sorted[j].Name())
		}
		return wi > wj
	})
	return sorted
}
