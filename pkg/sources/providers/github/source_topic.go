package github

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/defeedco/defeed/pkg/sources/types"

	"github.com/defeedco/defeed/pkg/lib"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"
)

const TypeGithubTopic = "githubtopic"

// SourceTopic fetches repositories for a single GitHub topic (tag)
// It can return either trending repositories (by stars) or newly created repositories.
type SourceTopic struct {
	Topic string `json:"topic" validate:"required"`

	client *github.Client
	logger *zerolog.Logger
}

func (s *SourceTopic) Topics() []types.TopicTag {
	tags := make([]types.TopicTag, 0)
	parts := strings.SplitSeq(s.Topic, "-")
	for part := range parts {
		if tag, ok := types.WordToTopic(part); ok {
			tags = append(tags, tag)
		}
	}

	if len(tags) == 0 {
		// Hardcoded fallback
		return []types.TopicTag{
			types.TopicOpenSource,
			types.TopicDevTools,
		}
	}

	return tags
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

func (s *SourceTopic) Initialize(logger *zerolog.Logger, config *types.ProviderConfig) error {
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

func (s *SourceTopic) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	s.fetchTopicRepositories(ctx, since, feed, errs)
}

func (s *SourceTopic) fetchTopicRepositories(ctx context.Context, _ activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	// minTrendingStars could be set based on the popularity of the topic (more popular topics => higher popularity thresholds)
	minTrendingStars := 200
	perPage := 200
	pageLimit := 2
	// Note: Do not filter by creation date, since popular repositories can be arbitrary old, but only recently gain popularity.
	query := fmt.Sprintf("topic:%s stars:>%d", s.Topic, minTrendingStars)

	s.logger.Debug().
		Str("topic", s.Topic).
		Str("query", query).
		Msg("Searching GitHub repositories by topic")

	page := 1
	for {
		result, _, err := s.client.Search.Repositories(ctx, query, &github.SearchOptions{
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

		s.logger.Debug().
			Int("count", len(result.Repositories)).
			Int("page", page).
			Str("query", query).
			Msg("Fetched repositories for topic")

		for _, repo := range result.Repositories {
			// Safety checks
			if repo.Owner == nil || repo.Owner.Login == nil || repo.Name == nil {
				continue
			}
			activity := &Repository{
				Repository:            repo,
				SourceID:              s.UID(),
				PopularityReachedDate: time.Now(),
			}
			feed <- activity
		}

		if len(result.Repositories) == 0 || page >= pageLimit {
			break
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
	// PopularityReachedDate is the date when the repository reached the stars threshold
	PopularityReachedDate time.Time `json:"popularity_reached_date"`
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
	return a.PopularityReachedDate
}

func (a *Repository) UpvotesCount() int {
	return a.Repository.GetStargazersCount()
}

func (a *Repository) DownvotesCount() int {
	return -1
}

func (a *Repository) CommentsCount() int {
	return a.Repository.GetOpenIssuesCount()
}

func (a *Repository) AmplificationCount() int {
	return a.Repository.GetForksCount()
}

func (a *Repository) SocialScore() float64 {
	stars := float64(a.UpvotesCount())
	forks := float64(a.AmplificationCount())
	issues := float64(a.CommentsCount())

	starsWeight := 0.5
	forksWeight := 0.3
	issuesWeight := 0.2

	maxStars := 10000.0
	maxForks := 2000.0
	maxIssues := 500.0

	normalizedStars := math.Min(stars/maxStars, 1.0)
	normalizedForks := math.Min(forks/maxForks, 1.0)
	normalizedIssues := math.Min(issues/maxIssues, 1.0)

	socialScore := (normalizedStars * starsWeight) + (normalizedForks * forksWeight) + (normalizedIssues * issuesWeight)

	return math.Min(socialScore, 1.0)
}
