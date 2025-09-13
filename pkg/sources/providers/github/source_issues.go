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

const TypeGithubIssues = "githubissues"

type SourceIssues struct {
	Owner  string `json:"owner" validate:"required"`
	Repo   string `json:"repo" validate:"required"`
	client *github.Client
	logger *zerolog.Logger
}

func NewIssuesSource() *SourceIssues {
	return &SourceIssues{}
}

func (s *SourceIssues) UID() activitytypes.TypedUID {
	return &TypedUID{
		Typ:   TypeGithubIssues,
		Owner: s.Owner,
		Repo:  s.Repo,
	}
}

func (s *SourceIssues) Name() string {
	return fmt.Sprintf("Issues on %s/%s", s.Owner, s.Repo)
}

func (s *SourceIssues) Description() string {
	return fmt.Sprintf("Recent issue activity from %s/%s", s.Owner, s.Repo)
}

func (s *SourceIssues) URL() string {
	return fmt.Sprintf("https://github.com/%s/%s/issues", s.Owner, s.Repo)
}

func (s *SourceIssues) Icon() string {
	return "https://github.com/favicon.ico"
}

func (s *SourceIssues) Topics() []sourcetypes.TopicTag {
	return []sourcetypes.TopicTag{sourcetypes.TopicDevTools, sourcetypes.TopicOpenSource}
}

func (s *SourceIssues) MarshalJSON() ([]byte, error) {
	type Alias SourceIssues
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  TypeGithubIssues,
	})
}

func (s *SourceIssues) UnmarshalJSON(data []byte) error {
	type Alias SourceIssues
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

type Issue struct {
	Owner    string        `json:"owner"`
	Repo     string        `json:"repo"`
	Issue    *github.Issue `json:"issue"`
	SourceID *TypedUID     `json:"source_id"`
}

func NewIssue() *Issue {
	return &Issue{}
}

func (i *Issue) SourceType() string {
	return TypeGithubIssues
}

func (i *Issue) MarshalJSON() ([]byte, error) {
	type Alias Issue
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(i),
	})
}

func (i *Issue) UnmarshalJSON(data []byte) error {
	type Alias Issue
	aux := &struct {
		*Alias
		SourceID *TypedUID `json:"source_id"`
	}{
		Alias: (*Alias)(i),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.SourceID == nil {
		return fmt.Errorf("source_id is required")
	}

	i.SourceID = aux.SourceID
	return nil
}

func (i *Issue) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(TypeGithubIssues, fmt.Sprintf("%d", i.Issue.GetNumber()))
}

func (i *Issue) SourceUID() activitytypes.TypedUID {
	return i.SourceID
}

func (i *Issue) Title() string {
	return i.Issue.GetTitle()
}

func (i *Issue) Body() string {
	return i.Issue.GetBody()
}

func (i *Issue) URL() string {
	return i.Issue.GetHTMLURL()
}

func (i *Issue) ImageURL() string {
	return fmt.Sprintf(
		"https://opengraph.githubassets.com/%d/%s/%s/issues/%d",
		i.Issue.UpdatedAt.Unix(),
		i.Owner,
		i.Repo,
		i.Issue.GetNumber(),
	)
}

func (i *Issue) CreatedAt() time.Time {
	return i.Issue.GetUpdatedAt().Time
}

func (s *SourceIssues) Initialize(logger *zerolog.Logger, config *sourcetypes.ProviderConfig) error {
	if err := lib.ValidateStruct(s); err != nil {
		return err
	}

	if config.GithubAPIKey != "" {
		s.client = github.NewClient(nil).WithAuthToken(config.GithubAPIKey)
	} else {
		s.client = github.NewClient(nil)
	}

	s.logger = logger

	return nil
}

func (s *SourceIssues) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	s.fetchIssueActivities(ctx, since, feed, errs)
}

func (s *SourceIssues) fetchIssueActivities(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	// TODO: When since is non-empty, it always fetches the one last issue we've already seen
	issues, _, err := s.client.Issues.ListByRepo(ctx, s.Owner, s.Repo, &github.IssueListByRepoOptions{
		State:     "all",
		Sort:      "updated",
		Direction: "desc",
		Since:     sinceTime,
	})
	if err != nil {
		errs <- fmt.Errorf("list issues: %w", err)
		return
	}

	s.logger.Debug().
		Str("repository", fmt.Sprintf("%s/%s", s.Owner, s.Repo)).
		Time("since", sinceTime).
		Int("count", len(issues)).
		Msg("Fetched issues")

	for _, issue := range issues {
		activity := &Issue{
			Issue:    issue,
			SourceID: s.UID().(*TypedUID),
			Owner:    s.Owner,
			Repo:     s.Repo,
		}
		feed <- activity
	}
}
