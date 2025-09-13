package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources/types"
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

	// Nothing found
	return []types.Source{}, nil
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

	if config.GithubAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.GithubAPIKey)
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
