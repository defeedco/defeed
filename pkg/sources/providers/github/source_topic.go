package github

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/types"
	"os"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	activitytypes "github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"
)

const TypeGithubTopic = "githubtopic"

// SourceTopic fetches repositories for a single GitHub topic (tag)
// It can return either trending repositories (by stars) or newly created repositories.
type SourceTopic struct {
	Topic string `json:"topic" validate:"required"`
	Token string `json:"token"`

	client *github.Client
	logger *zerolog.Logger
}

func (s *SourceTopic) Topics() []types.TopicTag {
	return []types.TopicTag{
		types.TopicOpenSource,
		types.TopicDevTools,
	}
}

func NewSourceTopic() *SourceTopic {
	return &SourceTopic{}
}

func (s *SourceTopic) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(TypeGithubTopic, s.Topic)
}

func (s *SourceTopic) Name() string {
	return fmt.Sprintf("Topic #%s", s.Topic)
}

func (s *SourceTopic) Description() string {
	return fmt.Sprintf("Trending repositories tagged #%s", s.Topic)
}

func (s *SourceTopic) URL() string {
	return fmt.Sprintf("https://github.com/topics/%s", s.Topic)
}

func (s *SourceTopic) Icon() string {
	return "https://github.com/favicon.ico"
}

func (s *SourceTopic) Initialize(logger *zerolog.Logger) error {
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

func (s *SourceTopic) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	s.fetchTopicRepositories(ctx, since, feed, errs)
}

func (s *SourceTopic) fetchTopicRepositories(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	// Hyperparameters - adjust according to the rate limits available
	minTrendingStars := 1000
	perPage := 100
	pageLimit := 1
	mode := "trending"

	var sinceDate string
	if since != nil {
		sinceDate = since.CreatedAt().Format(time.DateOnly)
	} else {
		// Default look-back window
		switch mode {
		case "new":
			sinceDate = time.Now().AddDate(0, 0, -14).Format(time.DateOnly)
		default:
			sinceDate = time.Now().AddDate(0, -1, 0).Format(time.DateOnly)
		}
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString("topic:")
	queryBuilder.WriteString(s.Topic)
	queryBuilder.WriteString(" ")
	queryBuilder.WriteString("stars:>")
	queryBuilder.WriteString(fmt.Sprintf("%d", minTrendingStars))

	if sinceDate != "" {
		queryBuilder.WriteString(" ")
		queryBuilder.WriteString("created:>")
		queryBuilder.WriteString(sinceDate)
	}

	searchQuery := queryBuilder.String()

	s.logger.Debug().
		Str("topic", s.Topic).
		Str("query", searchQuery).
		Msg("Searching GitHub repositories by topic")

	page := 1
	for {
		result, _, err := s.client.Search.Repositories(ctx, searchQuery, &github.SearchOptions{
			ListOptions: github.ListOptions{
				PerPage: perPage,
				Page:    page,
			},
			Order: "desc",
			Sort:  "created",
		})
		if err != nil {
			errs <- fmt.Errorf("search repositories: %w", err)
			return
		}

		if len(result.Repositories) == 0 || page >= pageLimit {
			break
		}

		s.logger.Debug().
			Int("count", len(result.Repositories)).
			Int("page", page).
			Msg("Fetched repositories for topic")

		for _, repo := range result.Repositories {
			// Safety checks
			if repo.Owner == nil || repo.Owner.Login == nil || repo.Name == nil {
				continue
			}
			activity := &Repository{
				Repository: repo,
				SourceID:   s.UID(),
			}
			feed <- activity
		}

		page++
	}
}

func (s *SourceTopic) MarshalJSON() ([]byte, error) {
	type Alias SourceTopic
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  TypeGithubTopic,
	})
}

func (s *SourceTopic) UnmarshalJSON(data []byte) error {
	type Alias SourceTopic
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

// Repository represents a repository result as an activity
type Repository struct {
	Repository *github.Repository     `json:"repository"`
	SourceID   activitytypes.TypedUID `json:"source_id"`
}

func NewRepository() *Repository {
	return &Repository{}
}

func (a *Repository) SourceType() string {
	return a.SourceID.Type()
}

func (a *Repository) MarshalJSON() ([]byte, error) {
	type Alias Repository
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(a),
	})
}

func (a *Repository) UnmarshalJSON(data []byte) error {
	type Alias Repository
	aux := &struct {
		*Alias
		SourceID *lib.TypedUID `json:"source_id"`
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.SourceID == nil {
		return fmt.Errorf("source_id is required")
	}
	a.SourceID = aux.SourceID
	return nil
}

func (a *Repository) UID() activitytypes.TypedUID {
	fullName := a.Repository.GetFullName()
	if fullName == "" {
		fullName = fmt.Sprintf("%s/%s", a.Repository.GetOwner().GetLogin(), a.Repository.GetName())
	}
	return lib.NewTypedUID(TypeGithubTopic, fullName)
}

func (a *Repository) SourceUID() activitytypes.TypedUID {
	return a.SourceID
}

func (a *Repository) Title() string {
	if a.Repository.FullName != nil {
		return *a.Repository.FullName
	}
	return a.Repository.GetName()
}

func (a *Repository) Body() string {
	return a.Repository.GetDescription()
}

func (a *Repository) URL() string {
	return a.Repository.GetHTMLURL()
}

func (a *Repository) ImageURL() string {
	owner := a.Repository.GetOwner().GetLogin()
	repo := a.Repository.GetName()
	created := a.CreatedAt().Unix()
	return fmt.Sprintf("https://opengraph.githubassets.com/%d/%s/%s", created, owner, repo)
}

func (a *Repository) CreatedAt() time.Time {
	if a.Repository.CreatedAt != nil {
		return a.Repository.GetCreatedAt().Time
	}
	if a.Repository.PushedAt != nil {
		return a.Repository.GetPushedAt().Time
	}
	if a.Repository.UpdatedAt != nil {
		return a.Repository.GetUpdatedAt().Time
	}
	return time.Now()
}
