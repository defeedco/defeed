package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"
)

const TypeGithubIssues = "github:issues"

type SourceIssues struct {
	Repository string `json:"repository" validate:"required,contains=/"`
	Token      string `json:"token"`
	client     *github.Client
	logger     *zerolog.Logger
}

func NewIssuesSource() *SourceIssues {
	return &SourceIssues{}
}

func (s *SourceIssues) UID() lib.TypedUID {
	return lib.NewTypedUID(TypeGithubIssues, s.Repository)
}

func (s *SourceIssues) Name() string {
	return fmt.Sprintf("Issues on %s", s.Repository)
}

func (s *SourceIssues) Description() string {
	return fmt.Sprintf("Recent issue activity from %s", s.Repository)
}

func (s *SourceIssues) URL() string {
	return fmt.Sprintf("https://github.com/%s", s.Repository)
}

func (s *SourceIssues) Validate() []error { return lib.ValidateStruct(s) }

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
	Repository string        `json:"repository"`
	Issue      *github.Issue `json:"issue"`
	SourceID   lib.TypedUID  `json:"source_id"`
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
	}{
		Alias: (*Alias)(i),
	}
	return json.Unmarshal(data, &aux)
}

func (i *Issue) UID() lib.TypedUID {
	return lib.NewTypedUID(TypeGithubIssues, fmt.Sprintf("%d", i.Issue.GetNumber()))
}

func (i *Issue) SourceUID() lib.TypedUID {
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
		"https://opengraph.githubassets.com/%d/%s/issues/%d",
		i.Issue.UpdatedAt.Unix(),
		i.Repository,
		*i.Issue.Number,
	)
}

func (i *Issue) CreatedAt() time.Time {
	return i.Issue.GetUpdatedAt().Time
}

func (s *SourceIssues) Initialize(logger *zerolog.Logger) error {
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

func (s *SourceIssues) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	s.fetchIssueActivities(ctx, s.client, s.Repository, since, feed, errs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchIssueActivities(ctx, s.client, s.Repository, since, feed, errs)
		}
	}
}

func (s *SourceIssues) fetchIssueActivities(ctx context.Context, client *github.Client, repository string, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		errs <- fmt.Errorf("invalid Repository format: %s", repository)
		return
	}
	owner, repo := parts[0], parts[1]

	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	// TODO: When since is non-empty, it always fetches the one last issue we've already seen
	issues, _, err := client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
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
		Str("repository", repository).
		Time("since", sinceTime).
		Int("count", len(issues)).
		Msg("Fetched issues")

	for _, issue := range issues {
		activity := &Issue{
			Issue:      issue,
			SourceID:   s.UID(),
			Repository: s.Repository,
		}
		feed <- activity
	}
}
