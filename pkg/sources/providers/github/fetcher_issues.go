package github

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/glanceapp/glance/pkg/sources/fetcher"
	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"
)

// IssuesFetcher implements preset search functionality for GitHub Issues
type IssuesFetcher struct {
	Logger *zerolog.Logger
}

func NewIssuesFetcher(logger *zerolog.Logger) *IssuesFetcher {
	return &IssuesFetcher{
		Logger: logger,
	}
}

func (f *IssuesFetcher) Search(ctx context.Context, query string) ([]fetcher.Source, error) {
	// TODO: Move to config struct
	token := os.Getenv("GITHUB_TOKEN")
	var client *github.Client
	if token != "" {
		client = github.NewClient(nil).WithAuthToken(token)
	} else {
		client = github.NewClient(nil)
	}

	var searchQuery string
	if query == "" {
		searchQuery = trendingRepositoriesQuery()
	} else {
		searchQuery = query
	}

	searchResult, _, err := client.Search.Repositories(ctx, searchQuery, &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: 10,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("search repositories: %w", err)
	}

	var sources []fetcher.Source
	for _, repo := range searchResult.Repositories {
		if repo.FullName == nil {
			continue
		}

		source := &SourceIssues{
			Repository: *repo.FullName,
		}
		sources = append(sources, source)
	}

	f.Logger.Debug().
		Str("original_query", query).
		Str("search_query", searchQuery).
		Int("results", len(sources)).
		Msg("GitHub Issues fetcher found repositories")

	return sources, nil
}

// trendingRepositoriesQuery returns an approximate query for trending repositories
func trendingRepositoriesQuery() string {
	oneMonthAgo := time.Now().AddDate(0, -1, 0).Format(time.DateOnly)
	return fmt.Sprintf("created:>%s stars:>1000 sort:stars-desc", oneMonthAgo)
}
