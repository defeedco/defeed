package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/glanceapp/glance/pkg/utils"

	"github.com/google/go-github/v72/github"
)

const TypeGithubIssues = "github-issues"

type SourceIssues struct {
	Repository string `json:"repository" validate:"required,contains=/"`
	Token      string `json:"token"`
	client     *github.Client
}

func NewIssuesSource() *SourceIssues {
	return &SourceIssues{}
}

func (s *SourceIssues) UID() string {
	return fmt.Sprintf("%s/%s", s.Type(), s.Repository)
}

func (s *SourceIssues) Name() string {
	return fmt.Sprintf("Issue Activity (%s)", s.Repository)
}

func (s *SourceIssues) URL() string {
	return fmt.Sprintf("https://github.com/%s", s.Repository)
}

func (s *SourceIssues) Type() string {
	return TypeGithubIssues
}

func (s *SourceIssues) Validate() []error { return utils.ValidateStruct(s) }

func (s *SourceIssues) MarshalJSON() ([]byte, error) {
	type Alias SourceIssues
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  s.Type(),
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
	SourceID   string        `json:"source_id"`
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

func (i *Issue) UID() string {
	return fmt.Sprintf("issue-%d", i.Issue.GetNumber())
}

func (i *Issue) SourceUID() string {
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

func (s *SourceIssues) Initialize() error {
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

func (s *SourceIssues) Stream(ctx context.Context, feed chan<- types.Activity, errs chan<- error) {
	activities, err := s.fetchIssueActivities(ctx, s.client, s.Repository)

	if err != nil {
		errs <- err
		return
	}

	for _, activity := range activities {
		feed <- activity
	}
}

func (s *SourceIssues) fetchIssueActivities(ctx context.Context, client *github.Client, repository string) ([]*Issue, error) {
	activities := make([]*Issue, 0)

	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid Repository format: %s", repository)
	}
	owner, repo := parts[0], parts[1]

	issues, _, err := client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
		State:       "all",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 10},
	})
	if err != nil {
		return nil, err
	}

	for _, issue := range issues {
		activities = append(activities, &Issue{
			Issue:      issue,
			SourceID:   s.UID(),
			Repository: s.Repository,
		})
	}

	return activities, nil
}
