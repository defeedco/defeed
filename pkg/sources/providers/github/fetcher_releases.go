package github

import (
	"context"
	"fmt"
	"os"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/types"

	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"
)

// ReleasesFetcher implements preset search functionality for GitHub Releases
type ReleasesFetcher struct {
	Logger *zerolog.Logger
}

func NewReleasesFetcher(logger *zerolog.Logger) *ReleasesFetcher {
	return &ReleasesFetcher{
		Logger: logger,
	}
}

func (f *ReleasesFetcher) SourceType() string {
	return TypeGithubReleases
}

func (f *ReleasesFetcher) FindByID(ctx context.Context, id lib.TypedUID) (types.Source, error) {
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

	return &SourceRelease{
		Owner:            *repo.Owner.Login,
		Repo:             *repo.Name,
		IncludePreleases: false,
	}, nil
}

func (f *ReleasesFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
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

	var sources []types.Source
	for _, repo := range searchResult.Repositories {
		if repo.FullName == nil {
			continue
		}

		source := &SourceRelease{
			Owner:            *repo.Owner.Login,
			Repo:             *repo.Name,
			IncludePreleases: false,
		}
		sources = append(sources, source)
	}

	f.Logger.Debug().
		Str("query", query).
		Int("results", len(sources)).
		Msg("GitHub Releases fetcher found repositories")

	return sources, nil
}
