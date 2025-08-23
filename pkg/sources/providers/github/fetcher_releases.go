package github

import (
	"context"
	"fmt"
	"os"

	"github.com/glanceapp/glance/pkg/sources/fetcher"
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

func (f *ReleasesFetcher) Search(ctx context.Context, query string) ([]fetcher.Source, error) {
	if query == "" {
		return nil, nil
	}

	token := os.Getenv("GITHUB_TOKEN")
	var client *github.Client
	if token != "" {
		client = github.NewClient(nil).WithAuthToken(token)
	} else {
		client = github.NewClient(nil)
	}

	searchResult, _, err := client.Search.Repositories(ctx, query, &github.SearchOptions{
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

		source := &SourceRelease{
			Repository:       *repo.FullName,
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
