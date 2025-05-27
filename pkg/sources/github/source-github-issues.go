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

type SourceIssues struct {
	Repository string `json:"repository"`
	Token      string `json:"token"`
	client     *github.Client
}

func NewIssuesSource() *SourceIssues {
	return &SourceIssues{}
}

func (s *SourceIssues) UID() string {
	return fmt.Sprintf("issues/%s", s.Repository)
}

func (s *SourceIssues) Name() string {
	return fmt.Sprintf("Issue Activity (%s)", s.Repository)
}

func (s *SourceIssues) URL() string {
	return fmt.Sprintf("https://github.com/%s", s.Repository)
}

type issueActivity struct {
	repository string
	raw        *github.Issue
	sourceUID  string
}

func (i issueActivity) UID() string {
	return fmt.Sprintf("issue-%d", i.raw.GetNumber())
}

func (i issueActivity) SourceUID() string {
	return i.sourceUID
}

func (i issueActivity) Title() string {
	return i.raw.GetTitle()
}

func (i issueActivity) Body() string {
	return i.raw.GetBody()
}

func (i issueActivity) URL() string {
	return i.raw.GetHTMLURL()
}

func (i issueActivity) ImageURL() string {
	return fmt.Sprintf(
		"https://opengraph.githubassets.com/%d/%s/issues/%d",
		i.raw.UpdatedAt.Unix(),
		i.repository,
		*i.raw.Number,
	)
}

func (i issueActivity) CreatedAt() time.Time {
	return i.raw.GetUpdatedAt().Time
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

func (s *SourceIssues) Stream(ctx context.Context, feed chan<- common.Activity, errs chan<- error) {
	activities, err := s.fetchIssueActivities(ctx, s.client, s.Repository)

	if err != nil {
		errs <- err
		return
	}

	for _, activity := range activities {
		feed <- activity
	}
}

func (s *SourceIssues) fetchIssueActivities(ctx context.Context, client *github.Client, repository string) ([]issueActivity, error) {
	activities := make([]issueActivity, 0)

	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s", repository)
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
		activities = append(activities, issueActivity{
			raw:        issue,
			sourceUID:  s.UID(),
			repository: s.Repository,
		})
	}

	return activities, nil
}
