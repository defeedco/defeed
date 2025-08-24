package github

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/types"

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

func (f *IssuesFetcher) SourceType() string {
	return TypeGithubIssues
}

func (f *IssuesFetcher) FindByID(ctx context.Context, id lib.TypedUID) (types.Source, error) {
	// TODO: Move to Initialize() func and read from Config struct (add to providers/config.go)
	token := os.Getenv("GITHUB_TOKEN")
	var client *github.Client
	if token != "" {
		client = github.NewClient(nil).WithAuthToken(token)
	} else {
		client = github.NewClient(nil)
	}

	ghUID, ok := id.(*TypedUID)
	if !ok {
		return nil, fmt.Errorf("not a GitHub typed UID: %s", id.String())
	}

	repo, _, err := client.Repositories.Get(ctx, ghUID.Owner, ghUID.Repo)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	return &SourceIssues{
		Owner: *repo.Owner.Login,
		Repo:  *repo.Name,
	}, nil
}

func (f *IssuesFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	// TODO: Move to Initialize() func and read from Config struct (add to providers/config.go)
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

	var sources []types.Source
	for _, repo := range searchResult.Repositories {
		if repo.FullName == nil {
			continue
		}

		source := &SourceIssues{
			Owner: *repo.Owner.Login,
			Repo:  *repo.Name,
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
