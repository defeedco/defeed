package github

import (
	"context"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/common"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v72/github"
)

type SourceRelease struct {
	Repository       string `json:"repository"`
	Token            string `json:"token"`
	IncludePreleases bool   `json:"include_prereleases"`
	client           *github.Client
}

func NewReleaseSource() *SourceRelease {
	return &SourceRelease{
		IncludePreleases: false,
	}
}

func (s *SourceRelease) UID() string {
	return fmt.Sprintf("releases/%s", s.Repository)
}

func (s *SourceRelease) Name() string {
	return fmt.Sprintf("Releases (%s)", s.Repository)
}

func (s *SourceRelease) URL() string {
	return fmt.Sprintf("https://github.com/%s", s.Repository)
}

func (s *SourceRelease) Stream(ctx context.Context, feed chan<- common.Activity, errs chan<- error) {
	release, err := s.fetchLatestGithubRelease(ctx)

	if err != nil {
		errs <- err
		return
	}

	feed <- release
}

func (s *SourceRelease) Initialize() error {

	token := s.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	if token != "" {
		s.client = github.NewClient(nil).WithAuthToken(token)
	} else {
		s.client = github.NewClient(nil)
	}

	return nil
}

type githubRelease struct {
	repository string
	raw        *github.RepositoryRelease
	sourceUID  string
}

func (a githubRelease) UID() string {
	return fmt.Sprintf("%d", a.raw.GetID())
}

func (a githubRelease) SourceUID() string {
	return a.sourceUID
}

func (a githubRelease) Title() string {
	return a.raw.GetName()
}

func (a githubRelease) Body() string {
	return a.raw.GetBody()
}

func (a githubRelease) URL() string {
	return a.raw.GetHTMLURL()
}

func (a githubRelease) ImageURL() string {
	return fmt.Sprintf(
		"https://opengraph.githubassets.com/%d/%s/releases/tag/%s",
		a.raw.CreatedAt.Unix(),
		a.repository,
		*a.raw.TagName,
	)
}

func (a githubRelease) CreatedAt() time.Time {
	return a.raw.GetPublishedAt().Time
}

func (s *SourceRelease) fetchLatestGithubRelease(ctx context.Context) (*githubRelease, error) {
	parts := strings.Split(s.Repository, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s", s.Repository)
	}
	owner, repo := parts[0], parts[1]

	var release *github.RepositoryRelease
	var err error

	if !s.IncludePreleases {
		release, _, err = s.client.Repositories.GetLatestRelease(ctx, owner, repo)
	} else {
		releases, _, err := s.client.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{PerPage: 1})
		if err != nil {
			return nil, err
		}
		if len(releases) == 0 {
			return nil, fmt.Errorf("no releases found for repository %s", s.Repository)
		}
		release = releases[0]
	}

	if err != nil {
		return nil, err
	}

	return &githubRelease{
		raw:        release,
		repository: s.Repository,
		sourceUID:  s.UID(),
	}, nil
}
