package presets

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/glanceapp/glance/pkg/sources"
	"github.com/glanceapp/glance/pkg/sources/providers/github"
	"github.com/glanceapp/glance/pkg/sources/providers/hackernews"
	"github.com/glanceapp/glance/pkg/sources/providers/lobsters"
	"github.com/glanceapp/glance/pkg/sources/providers/mastodon"
	"github.com/glanceapp/glance/pkg/sources/providers/reddit"
	"github.com/glanceapp/glance/pkg/sources/providers/rss"
	gogithub "github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"
)

// Source: https://raw.githubusercontent.com/tuan3w/awesome-tech-rss/refs/heads/main/feeds.opml
//
//go:embed awesome-tech-rss-list.opml
var awesomeTechRSSListRaw string

// Fetcher interface allows source types to provide preset/search functionality.
type Fetcher interface {
	Search(ctx context.Context, query string) ([]sources.Source, error)
}

// Registry manages available source configurations through fetchers.
type Registry struct {
	fetchers map[string]Fetcher
	logger   *zerolog.Logger
}

func NewRegistry(logger *zerolog.Logger) *Registry {
	return &Registry{
		fetchers: make(map[string]Fetcher),
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

	// Register fetchers for each source type
	r.fetchers["github:issues"] = &githubIssuesFetcher{logger: r.logger}
	r.fetchers["github:releases"] = &githubReleasesFetcher{logger: r.logger}
	r.fetchers["rss:feed"] = &rssFetcher{opmlSources: opmlSources, logger: r.logger}
	r.fetchers["reddit:subreddit"] = &redditFetcher{logger: r.logger}
	r.fetchers["hackernews:posts"] = &hackerNewsFetcher{logger: r.logger}
	r.fetchers["lobsters:feed"] = &lobstersFetcher{logger: r.logger}
	r.fetchers["mastodon:account"] = &mastodonAccountFetcher{logger: r.logger}
	r.fetchers["mastodon:tag"] = &mastodonTagFetcher{logger: r.logger}

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
	fetcher, exists := r.fetchers[sourceType]
	if !exists {
		return nil, fmt.Errorf("no fetcher available for source type: %s", sourceType)
	}

	sources, err := fetcher.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search failed for type %s: %w", sourceType, err)
	}

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

	for sourceType, fetcher := range r.fetchers {
		sources, err := fetcher.Search(ctx, "")
		if err != nil {
			r.logger.Warn().
				Err(err).
				Str("source_type", sourceType).
				Msg("failed to get presets for source type")
			continue
		}
		result[sourceType] = sources
	}

	return result, nil
}

// Fetcher implementations for each source type

// GitHub Issues Fetcher
type githubIssuesFetcher struct {
	logger *zerolog.Logger
}

func (f *githubIssuesFetcher) Search(ctx context.Context, query string) ([]sources.Source, error) {
	if query == "" {
		return nil, nil
	}

	token := os.Getenv("GITHUB_TOKEN")
	var client *gogithub.Client
	if token != "" {
		client = gogithub.NewClient(nil).WithAuthToken(token)
	} else {
		client = gogithub.NewClient(nil)
	}

	searchResult, _, err := client.Search.Repositories(ctx, query, &gogithub.SearchOptions{
		ListOptions: gogithub.ListOptions{
			PerPage: 10,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("search repositories: %w", err)
	}

	var sources []sources.Source
	for _, repo := range searchResult.Repositories {
		if repo.FullName == nil {
			continue
		}

		source := &github.SourceIssues{
			Repository: *repo.FullName,
		}
		sources = append(sources, source)
	}

	f.logger.Debug().
		Str("query", query).
		Int("results", len(sources)).
		Msg("GitHub Issues fetcher found repositories")

	return sources, nil
}

// GitHub Releases Fetcher
type githubReleasesFetcher struct {
	logger *zerolog.Logger
}

func (f *githubReleasesFetcher) Search(ctx context.Context, query string) ([]sources.Source, error) {
	if query == "" {
		return nil, nil
	}

	token := os.Getenv("GITHUB_TOKEN")
	var client *gogithub.Client
	if token != "" {
		client = gogithub.NewClient(nil).WithAuthToken(token)
	} else {
		client = gogithub.NewClient(nil)
	}

	searchResult, _, err := client.Search.Repositories(ctx, query, &gogithub.SearchOptions{
		ListOptions: gogithub.ListOptions{
			PerPage: 10,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("search repositories: %w", err)
	}

	var sources []sources.Source
	for _, repo := range searchResult.Repositories {
		if repo.FullName == nil {
			continue
		}

		source := &github.SourceRelease{
			Repository:       *repo.FullName,
			IncludePreleases: false,
		}
		sources = append(sources, source)
	}

	f.logger.Debug().
		Str("query", query).
		Int("results", len(sources)).
		Msg("GitHub Releases fetcher found repositories")

	return sources, nil
}

// RSS Fetcher
type rssFetcher struct {
	opmlSources []sources.Source
	logger      *zerolog.Logger
}

func (f *rssFetcher) Search(ctx context.Context, query string) ([]sources.Source, error) {
	if query == "" {
		return f.opmlSources, nil
	}

	query = strings.ToLower(query)
	var matchingSources []sources.Source

	for _, source := range f.opmlSources {
		rssSource, ok := source.(*rss.SourceFeed)
		if !ok {
			continue
		}

		title := strings.ToLower(rssSource.Title)
		url := strings.ToLower(rssSource.FeedURL)

		if strings.Contains(title, query) || strings.Contains(url, query) {
			matchingSources = append(matchingSources, source)
		}
	}

	f.logger.Debug().
		Str("query", query).
		Int("total_opml", len(f.opmlSources)).
		Int("matches", len(matchingSources)).
		Msg("RSS fetcher searched OPML sources")

	return matchingSources, nil
}

// Reddit Fetcher
type redditFetcher struct {
	logger *zerolog.Logger
}

var popularSubreddits = []struct {
	name        string
	description string
}{
	{"programming", "Computer programming discussions"},
	{"MachineLearning", "Machine learning research and discussions"},
	{"javascript", "JavaScript programming language"},
	{"reactjs", "React.js library discussions"},
	{"Python", "Python programming language"},
	{"golang", "Go programming language"},
	{"rust", "Rust programming language"},
	{"webdev", "Web development discussions"},
	{"startups", "Startup discussions and news"},
	{"technology", "Technology news and discussions"},
	{"science", "Science news and discussions"},
	{"askscience", "Ask science questions"},
	{"datascience", "Data science discussions"},
	{"artificial", "Artificial intelligence discussions"},
	{"gamedev", "Game development discussions"},
	{"linux", "Linux operating system"},
	{"sysadmin", "System administration"},
	{"devops", "DevOps practices and tools"},
	{"cybersecurity", "Cybersecurity discussions"},
}

func (f *redditFetcher) Search(ctx context.Context, query string) ([]sources.Source, error) {
	query = strings.ToLower(query)
	var matchingSources []sources.Source

	for _, sub := range popularSubreddits {
		subName := strings.ToLower(sub.name)
		description := strings.ToLower(sub.description)

		if query == "" || strings.Contains(subName, query) || strings.Contains(description, query) {
			source := &reddit.SourceSubreddit{
				Subreddit: sub.name,
				SortBy:    "hot",
				TopPeriod: "day",
			}
			matchingSources = append(matchingSources, source)
		}
	}

	f.logger.Debug().
		Str("query", query).
		Int("matches", len(matchingSources)).
		Msg("Reddit fetcher found subreddits")

	return matchingSources, nil
}

// HackerNews Fetcher
type hackerNewsFetcher struct {
	logger *zerolog.Logger
}

var hackerNewsFeeds = []struct {
	name        string
	description string
}{
	{"new", "Latest posts from Hacker News"},
	{"top", "Top posts from Hacker News"},
	{"best", "Best posts from Hacker News"},
}

func (f *hackerNewsFetcher) Search(ctx context.Context, query string) ([]sources.Source, error) {
	query = strings.ToLower(query)
	var matchingSources []sources.Source

	for _, feed := range hackerNewsFeeds {
		feedName := strings.ToLower(feed.name)
		description := strings.ToLower(feed.description)

		if query == "" || strings.Contains(feedName, query) || strings.Contains(description, query) {
			source := &hackernews.SourcePosts{
				FeedName: feed.name,
			}
			matchingSources = append(matchingSources, source)
		}
	}

	f.logger.Debug().
		Str("query", query).
		Int("matches", len(matchingSources)).
		Msg("HackerNews fetcher found feeds")

	return matchingSources, nil
}

// Lobsters Fetcher
type lobstersFetcher struct {
	logger *zerolog.Logger
}

var lobstersFeeds = []struct {
	feedName    string
	description string
}{
	{"hottest", "Hottest posts from Lobsters"},
	{"newest", "Newest posts from Lobsters"},
}

func (f *lobstersFetcher) Search(ctx context.Context, query string) ([]sources.Source, error) {
	query = strings.ToLower(query)
	var matchingSources []sources.Source

	for _, feed := range lobstersFeeds {
		feedName := strings.ToLower(feed.feedName)
		description := strings.ToLower(feed.description)

		if query == "" || strings.Contains(feedName, query) || strings.Contains(description, query) {
			source := &lobsters.SourceFeed{
				InstanceURL: "https://lobste.rs",
				FeedName:    feed.feedName,
			}
			matchingSources = append(matchingSources, source)
		}
	}

	f.logger.Debug().
		Str("query", query).
		Int("matches", len(matchingSources)).
		Msg("Lobsters fetcher found feeds")

	return matchingSources, nil
}

// Mastodon Account Fetcher
type mastodonAccountFetcher struct {
	logger *zerolog.Logger
}

func (f *mastodonAccountFetcher) Search(ctx context.Context, query string) ([]sources.Source, error) {
	// Return template source for user customization
	source := &mastodon.SourceAccount{
		InstanceURL: "https://mastodon.social",
		Account:     "",
	}

	f.logger.Debug().
		Str("query", query).
		Msg("Mastodon Account fetcher - returning template for customization")

	return []sources.Source{source}, nil
}

// Mastodon Tag Fetcher
type mastodonTagFetcher struct {
	logger *zerolog.Logger
}

var popularMastodonTags = []struct {
	tag         string
	description string
}{
	{"programming", "Programming discussions"},
	{"technology", "Technology news and updates"},
	{"opensource", "Open source projects and discussions"},
	{"privacy", "Privacy-focused discussions"},
	{"security", "Cybersecurity topics"},
	{"linux", "Linux operating system"},
	{"javascript", "JavaScript programming"},
	{"python", "Python programming"},
	{"golang", "Go programming language"},
	{"rust", "Rust programming language"},
}

func (f *mastodonTagFetcher) Search(ctx context.Context, query string) ([]sources.Source, error) {
	query = strings.ToLower(query)
	var matchingSources []sources.Source

	for _, tag := range popularMastodonTags {
		tagName := strings.ToLower(tag.tag)
		description := strings.ToLower(tag.description)

		if query == "" || strings.Contains(tagName, query) || strings.Contains(description, query) {
			source := &mastodon.SourceTag{
				InstanceURL: "https://mastodon.social",
				Tag:         tag.tag,
			}
			matchingSources = append(matchingSources, source)
		}
	}

	// Also add empty template for user customization
	if query == "" || len(matchingSources) == 0 {
		source := &mastodon.SourceTag{
			InstanceURL: "https://mastodon.social",
			Tag:         "",
		}
		matchingSources = append(matchingSources, source)
	}

	f.logger.Debug().
		Str("query", query).
		Int("matches", len(matchingSources)).
		Msg("Mastodon Tag fetcher found tags")

	return matchingSources, nil
}
