package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/google/go-github/v72/github"
)

const TypeGithubReleases = "github-releases"

type SourceRelease struct {
	Repository       string `json:"Repository"`
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

func (s *SourceRelease) Type() string {
	return TypeGithubReleases
}

func (s *SourceRelease) Stream(ctx context.Context, feed chan<- types.Activity, errs chan<- error) {
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

func (s *SourceRelease) MarshalJSON() ([]byte, error) {
	type Alias SourceRelease
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  s.Type(),
	})
}

func (s *SourceRelease) UnmarshalJSON(data []byte) error {
	type Alias SourceRelease
	aux := &struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	return nil
}

type Release struct {
	Repository string                    `json:"repository"`
	Release    *github.RepositoryRelease `json:"release"`
	SourceID   string                    `json:"source_id"`
}

func NewRelease() *Release {
	return &Release{}
}

func (r *Release) SourceType() string {
	return TypeGithubReleases
}

func (r *Release) MarshalJSON() ([]byte, error) {
	type Alias Release
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}

func (r *Release) UnmarshalJSON(data []byte) error {
	type Alias Release
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	return json.Unmarshal(data, &aux)
}

func (r *Release) UID() string {
	return fmt.Sprintf("%d", r.Release.GetID())
}

func (r *Release) SourceUID() string {
	return r.SourceID
}

func (r *Release) Title() string {
	return r.Release.GetName()
}

func (r *Release) Body() string {
	return r.Release.GetBody()
}

func (r *Release) URL() string {
	return r.Release.GetHTMLURL()
}

func (r *Release) ImageURL() string {
	return fmt.Sprintf(
		"https://opengraph.githubassets.com/%d/%s/releases/tag/%s",
		r.Release.CreatedAt.Unix(),
		r.Repository,
		*r.Release.TagName,
	)
}

func (r *Release) CreatedAt() time.Time {
	return r.Release.GetPublishedAt().Time
}

func (s *SourceRelease) fetchLatestGithubRelease(ctx context.Context) (*Release, error) {
	parts := strings.Split(s.Repository, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid Repository format: %s", s.Repository)
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
			return nil, fmt.Errorf("no releases found for Repository %s", s.Repository)
		}
		release = releases[0]
	}

	if err != nil {
		return nil, err
	}

	return &Release{
		Release:    release,
		Repository: s.Repository,
		SourceID:   s.UID(),
	}, nil
}
