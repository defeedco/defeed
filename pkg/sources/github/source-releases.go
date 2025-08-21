package github

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/glanceapp/glance/pkg/lib"
	"os"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"
)

const TypeGithubReleases = "github-releases"

type SourceRelease struct {
	Repository       string `json:"repository" validate:"required,contains=/"`
	Token            string `json:"token"`
	IncludePreleases bool   `json:"includePrereleases"`
	client           *github.Client
	logger           *zerolog.Logger
}

func NewReleaseSource() *SourceRelease {
	return &SourceRelease{
		IncludePreleases: false,
	}
}

func (s *SourceRelease) UID() string {
	return fmt.Sprintf("%s/%s", s.Type(), s.Repository)
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

func (s *SourceRelease) Validate() []error { return lib.ValidateStruct(s) }

func (s *SourceRelease) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	s.fetchAndSendNewReleases(ctx, since, feed, errs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchAndSendNewReleases(ctx, since, feed, errs)
		}
	}
}

func (s *SourceRelease) fetchAndSendNewReleases(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	releases, err := s.fetchGithubReleases(ctx, since)
	if err != nil {
		errs <- err
		return
	}

	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	for _, release := range releases {
		if since == nil || release.CreatedAt().After(sinceTime) {
			feed <- release
		}
	}
}

func (s *SourceRelease) Initialize(logger *zerolog.Logger) error {

	token := s.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	if token != "" {
		s.client = github.NewClient(nil).WithAuthToken(token)
	} else {
		s.client = github.NewClient(nil)
	}

	s.logger = logger

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

func (s *SourceRelease) fetchGithubReleases(ctx context.Context, since types.Activity) ([]*Release, error) {
	parts := strings.Split(s.Repository, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid Repository format: %s", s.Repository)
	}
	owner, repo := parts[0], parts[1]

	sinceTime := time.Now().Add(-1 * time.Hour)
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	var result []*Release
	page := 1
outer:
	for {
		releases, _, err := s.client.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{
			PerPage: 10,
			Page:    page,
		})
		if err != nil {
			return nil, err
		}

		s.logger.Debug().
			Str("repository", s.Repository).
			Time("since", sinceTime).
			Int("count", len(releases)).
			Msg("Fetched releases")

		if len(releases) == 0 {
			break
		}

		for _, release := range releases {
			if !s.IncludePreleases && release.GetPrerelease() {
				continue
			}
			if release.GetPublishedAt().Before(sinceTime) {
				// Found the last release, stop looking for more
				break outer
			}
			result = append(result, &Release{
				Release:    release,
				Repository: s.Repository,
				SourceID:   s.UID(),
			})
		}
		page++
	}

	return result, nil
}
