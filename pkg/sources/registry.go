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

// SearchParams configures how sources are searched and ranked.
type SearchParams struct {
    Query     string
    Interests []string
}

// Search searches for sources from available fetchers
func (r *Registry) Search(ctx context.Context, params SearchParams) ([]types.Source, error) {
	g, gctx := errgroup.WithContext(ctx)

	g.SetLimit(len(r.fetchers))

	results := make([]types.Source, 0)
	for _, f := range r.fetchers {
		g.Go(func() error {
			res, err := f.Search(gctx, params.Query)
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

	// Normalize interests
	normalizedInterests := make([]string, 0, len(params.Interests))
	for _, it := range params.Interests {
		it = strings.TrimSpace(strings.ToLower(it))
		if it != "" {
			normalizedInterests = append(normalizedInterests, it)
		}
	}

	var filtered []types.Source
	switch {
	case params.Query != "":
		filtered = fuzzyReRank(results, params.Query)
	case len(normalizedInterests) > 0:
		filtered = filterAndRankByInterests(results, normalizedInterests)
	default:
		filtered = curatedDefaultSort(results)
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

// filterAndRankByInterests keeps only sources that overlap with the provided interests
// and ranks by number of matching tags, falling back to alphabetical for stability.
func filterAndRankByInterests(input []types.Source, interests []string) []types.Source {
	interestSet := make(map[string]struct{}, len(interests))
	for _, t := range interests {
		interestSet[t] = struct{}{}
	}

	type scored struct {
		s     types.Source
		score int
	}

	scoredList := make([]scored, 0, len(input))
	for _, s := range input {
		tags := deriveSourceTags(s)
		if len(tags) == 0 {
			continue
		}
		matches := 0
		for _, t := range tags {
			if _, ok := interestSet[strings.ToLower(strings.TrimSpace(t))]; ok {
				matches++
			}
		}
		if matches > 0 {
			scoredList = append(scoredList, scored{s: s, score: matches})
		}
	}

	if len(scoredList) == 0 {
		return curatedDefaultSort(input)
	}

	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].score == scoredList[j].score {
			return strings.ToLower(scoredList[i].s.Name()) < strings.ToLower(scoredList[j].s.Name())
		}
		return scoredList[i].score > scoredList[j].score
	})

	out := make([]types.Source, len(scoredList))
	for i, sc := range scoredList {
		out[i] = sc.s
	}
	return out
}

// deriveSourceTags attempts to infer topical tags for a source based on its
// type and available metadata. This avoids adding a new provider interface.
func deriveSourceTags(s types.Source) []string {
	tags := []string{"news"}

	switch s.UID().Type() {
	case reddit.TypeRedditSubreddit:
		tags = append(tags, "reddit")
		if sub, ok := s.(*reddit.SourceSubreddit); ok {
			tags = append(tags, strings.ToLower(sub.Subreddit))
			tags = append(tags, tokenizeToTags(sub.SubredditSummary)...)
		}
	case hackernews.TypeHackerNewsPosts:
		tags = append(tags, "hackernews", "technology", "programming")
	case lobsters.TypeLobstersTag:
		tags = append(tags, "lobsters", "technology", "programming")
		if lt, ok := s.(*lobsters.SourceTag); ok {
			tags = append(tags, strings.ToLower(lt.Tag))
			tags = append(tags, tokenizeToTags(lt.TagDescription)...)
		}
	case lobsters.TypeLobstersFeed:
		tags = append(tags, "lobsters", "technology")
	case github.TypeGithubIssues, github.TypeGithubReleases:
		tags = append(tags, "github", "software", "engineering", "programming")
	case rss.TypeRSSFeed:
		tags = append(tags, "rss")
		if rf, ok := s.(*rss.SourceFeed); ok {
			for _, t := range rf.Tags {
				if t != "" {
					tags = append(tags, strings.ToLower(strings.TrimSpace(t)))
				}
			}
			tags = append(tags, tokenizeToTags(rf.Title)...)
			tags = append(tags, tokenizeToTags(rf.AboutFeed)...)
		}
	case mastodon.TypeMastodonAccount, mastodon.TypeMastodonTag:
		tags = append(tags, "mastodon", "social")
	}

	// Expand and dedupe
	tags = expandTagSynonyms(tags)
	seen := map[string]struct{}{}
	uniq := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		uniq = append(uniq, t)
	}
	return uniq
}

func tokenizeToTags(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ToLower(s)
	for _, sep := range []string{",", ";", "/", "|", "-"} {
		s = strings.ReplaceAll(s, sep, " ")
	}
	parts := strings.Fields(s)
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) < 3 || len(p) > 30 {
			continue
		}
		tags = append(tags, p)
	}
	return tags
}

func expandTagSynonyms(tags []string) []string {
	if len(tags) == 0 {
		return tags
	}
	syn := map[string][]string{
		"ai":            {"artificialintelligence", "ml", "machinelearning"},
		"ml":            {"machinelearning", "ai"},
		"programming":   {"software", "engineering", "coding", "development", "dev"},
		"web":           {"webdev", "frontend", "backend", "javascript", "react"},
		"security":      {"infosec", "netsec", "appsec", "cybersecurity"},
		"linux":         {"unix", "opensource"},
		"opensource":    {"oss", "foss"},
		"startup":       {"startups", "entrepreneurship"},
		"data":          {"datascience", "analytics"},
		"datascience":   {"data", "machinelearning"},
		"devops":        {"infrastructure", "kubernetes", "containers"},
		"rss":           {"blog", "news"},
		"technology":    {"tech"},
	}
	out := make([]string, 0, len(tags)*2)
	out = append(out, tags...)
	for _, t := range tags {
		if alts, ok := syn[strings.ToLower(strings.TrimSpace(t))]; ok {
			out = append(out, alts...)
		}
	}
	return out
}
