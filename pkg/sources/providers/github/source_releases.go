package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"
)

const TypeGithubReleases = "githubreleases"

type SourceRelease struct {
	Owner            string `json:"owner" validate:"required"`
	Repo             string `json:"repo" validate:"required"`
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

func (s *SourceRelease) UID() types.TypedUID {
	return &TypedUID{
		Typ:   TypeGithubReleases,
		Owner: s.Owner,
		Repo:  s.Repo,
	}
}

func (s *SourceRelease) Name() string {
	return fmt.Sprintf("Releases on %s/%s", s.Owner, s.Repo)
}

func (s *SourceRelease) Description() string {
	if s.IncludePreleases {
		return fmt.Sprintf("All releases from %s/%s", s.Owner, s.Repo)
	}
	return fmt.Sprintf("Stable releases from %s/%s", s.Owner, s.Repo)
}

func (s *SourceRelease) URL() string {
	return fmt.Sprintf("https://github.com/%s/%s/releases", s.Owner, s.Repo)
}

func (s *SourceRelease) Icon() string {
	return "https://github.com/favicon.ico"
}

func (s *SourceRelease) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	s.fetchGithubReleases(ctx, since, feed, errs)
}

func (s *SourceRelease) Initialize(logger *zerolog.Logger) error {
	if err := lib.ValidateStruct(s); err != nil {
		return err
	}

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
		Type:  TypeGithubReleases,
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
	Owner    string                    `json:"owner"`
	Repo     string                    `json:"repo"`
	Release  *github.RepositoryRelease `json:"release"`
	SourceID *TypedUID                 `json:"source_id"`
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
		SourceID *TypedUID `json:"source_id"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.SourceID == nil {
		return fmt.Errorf("source_id is required")
	}

	r.SourceID = aux.SourceID
	return nil
}

func (r *Release) UID() types.TypedUID {
	return lib.NewTypedUID(TypeGithubReleases, fmt.Sprintf("%d", r.Release.GetID()))
}

func (r *Release) SourceUID() types.TypedUID {
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
		"https://opengraph.githubassets.com/%d/%s/%s/releases/tag/%s",
		r.Release.CreatedAt.Unix(),
		r.Owner,
		r.Repo,
		*r.Release.TagName,
	)
}

func (r *Release) CreatedAt() time.Time {
	return r.Release.GetPublishedAt().Time
}

func (s *SourceRelease) fetchGithubReleases(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	sinceTime := time.Now().Add(-1 * time.Hour)
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	page := 1
outer:
	for {
		releases, _, err := s.client.Repositories.ListReleases(ctx, s.Owner, s.Repo, &github.ListOptions{
			PerPage: 10,
			Page:    page,
		})
		if err != nil {
			errs <- err
			return
		}

		s.logger.Debug().
			Str("repository", fmt.Sprintf("%s/%s", s.Owner, s.Repo)).
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

			if since != nil && release.GetPublishedAt().Before(sinceTime) {
				continue
			}

			releaseActivity := &Release{
				Release:  release,
				Owner:    s.Owner,
				Repo:     s.Repo,
				SourceID: s.UID().(*TypedUID),
			}

			feed <- releaseActivity
		}
		page++
	}
}
