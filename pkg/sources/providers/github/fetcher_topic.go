package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	activitytypes "github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/types"
	"github.com/google/go-github/v72/github"
	"github.com/rs/zerolog"
)

// TopicFetcher provides preset/search functionality for GitHub Topic sources
type TopicFetcher struct {
	Logger *zerolog.Logger
}

func NewTopicFetcher(logger *zerolog.Logger) *TopicFetcher {
	return &TopicFetcher{Logger: logger}
}

func (f *TopicFetcher) SourceType() string {
	return TypeGithubTopic
}

func (f *TopicFetcher) FindByID(ctx context.Context, id activitytypes.TypedUID, config *types.ProviderConfig) (types.Source, error) {
	typedUID, ok := id.(*lib.TypedUID)
	if !ok {
		return nil, fmt.Errorf("not a typed UID: %s", id.String())
	}

	// See: SourceTopic.UID
	return &SourceTopic{
		Topic: typedUID.Identifiers[0],
	}, nil
}

func (f *TopicFetcher) Search(ctx context.Context, query string, config *types.ProviderConfig) ([]types.Source, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		// Cannot enumerate all topics; return empty
		return []types.Source{}, nil
	}

	suggestions, err := f.searchTopics(ctx, query, config)
	if err != nil {
		f.Logger.Debug().Err(err).Str("query", query).Msg("GitHub topic suggestions failed; falling back to existence check")
		return nil, fmt.Errorf("search topics: %w", err)
	}

	var sources []types.Source
	if len(suggestions) > 0 {
		for _, topic := range suggestions {
			sources = append(sources, &SourceTopic{Topic: topic})
		}

		f.Logger.Debug().
			Str("query", query).
			Int("suggestions", len(suggestions)).
			Int("results", len(sources)).
			Msg("GitHub Topic fetcher returning topic suggestions")
		return sources, nil
	}

	// Fallback: validate that the exact topic exists by checking for at least one repository
	exists, err := f.topicExistsByRepositorySearch(ctx, query, config)
	if err != nil {
		return nil, fmt.Errorf("check topic existence: %w", err)
	}
	if exists {
		return []types.Source{
			&SourceTopic{Topic: query},
		}, nil
	}

	// Nothing found
	return []types.Source{}, nil
}

// topicExistsByRepositorySearch checks if there is at least one repository for the given topic
func (f *TopicFetcher) topicExistsByRepositorySearch(ctx context.Context, topic string, config *types.ProviderConfig) (bool, error) {
	token := config.GithubAPIToken
	var client *github.Client
	if token != "" {
		client = github.NewClient(nil).WithAuthToken(token)
	} else {
		client = github.NewClient(nil)
	}

	query := fmt.Sprintf("topic:%s", topic)
	result, _, err := client.Search.Repositories(ctx, query, &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 1}})
	if err != nil {
		return false, err
	}
	return result.GetTotal() > 0, nil
}

// searchTopics queries the GitHub Topics search API to get topic suggestions
func (f *TopicFetcher) searchTopics(ctx context.Context, query string, config *types.ProviderConfig) ([]string, error) {
	// Build request
	base, _ := url.Parse("https://api.github.com/search/topics")
	q := base.Query()
	q.Set("q", query)
	q.Set("per_page", "5")
	base.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}

	// Required preview header for topics API
	req.Header.Set("Accept", "application/vnd.github.mercy-preview+json")

	if config.GithubAPIToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.GithubAPIToken)
	}

	type topicItem struct {
		Name string `json:"name"`
	}
	type topicResponse struct {
		TotalCount int         `json:"total_count"`
		Items      []topicItem `json:"items"`
	}

	resp, err := lib.DecodeJSONFromRequest[topicResponse](lib.DefaultHTTPClient, req)
	if err != nil {
		return nil, err
	}

	if resp.TotalCount == 0 || len(resp.Items) == 0 {
		return []string{}, nil
	}

	topics := make([]string, 0, len(resp.Items))
	seen := make(map[string]struct{})
	for _, it := range resp.Items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		topics = append(topics, name)
	}
	return topics, nil
}
