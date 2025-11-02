package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/defeedco/defeed/pkg/lib"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	sourcetypes "github.com/defeedco/defeed/pkg/sources/types"
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

func (s *SourceRelease) UID() activitytypes.TypedUID {
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

func (s *SourceRelease) Topics() []sourcetypes.TopicTag {
	return []sourcetypes.TopicTag{sourcetypes.TopicDevTools, sourcetypes.TopicOpenSource}
}

func (s *SourceRelease) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	s.fetchGithubReleases(ctx, since, feed, errs)
}

func (s *SourceRelease) Initialize(logger *zerolog.Logger, config *sourcetypes.ProviderConfig) error {
	if err := lib.ValidateStruct(s); err != nil {
		return err
	}

	token := s.Token
	if token == "" {
		token = config.GithubAPIKey
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
	Owner     string                    `json:"owner"`
	Repo      string                    `json:"repo"`
	Release   *github.RepositoryRelease `json:"release"`
	SourceIDs []*TypedUID               `json:"source_ids"`
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
		SourceIDs []*TypedUID `json:"source_ids"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.SourceIDs) == 0 {
		return fmt.Errorf("source_ids is required")
	}

	r.SourceIDs = aux.SourceIDs
	return nil
}

func (r *Release) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(TypeGithubReleases, fmt.Sprintf("%d", r.Release.GetID()))
}

func (r *Release) SourceUIDs() []activitytypes.TypedUID {
	result := make([]activitytypes.TypedUID, len(r.SourceIDs))
	for idx, uid := range r.SourceIDs {
		result[idx] = uid
	}
	return result
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

func (r *Release) UpvotesCount() int {
	return -1
}

func (r *Release) DownvotesCount() int {
	return -1
}

func (r *Release) CommentsCount() int {
	return -1
}

func (r *Release) AmplificationCount() int {
	return -1
}

func (r *Release) SocialScore() float64 {
	return -1
}

func (s *SourceRelease) fetchGithubReleases(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
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
				Release:   release,
				Owner:     s.Owner,
				Repo:      s.Repo,
				SourceIDs: []*TypedUID{s.UID().(*TypedUID)},
			}

			feed <- releaseActivity
		}
		page++
	}
}
