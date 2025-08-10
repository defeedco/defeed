package mastodon

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/mattn/go-mastodon"
)

const TypeMastodonTag = "mastodon-tag"

type SourceTag struct {
	InstanceURL string `json:"instanceUrl"`
	Tag         string `json:"tag"`
}

func NewSourceTag() *SourceTag {
	return &SourceTag{
		InstanceURL: "https://mastodon.social",
	}
}

func (s *SourceTag) UID() string {
	return fmt.Sprintf("%s/%s/%s", s.Type(), s.InstanceURL, s.Tag)
}

func (s *SourceTag) Name() string {
	return fmt.Sprintf("Mastodon (%s)", s.Tag)
}

func (s *SourceTag) URL() string {
	return fmt.Sprintf("%s/tags/%s", s.InstanceURL, s.Tag)
}

func (s *SourceTag) Type() string {
	return TypeMastodonTag
}

func (s *SourceTag) Initialize() error {
	if s.InstanceURL == "" {
		return fmt.Errorf("instance URL is required")
	}
	if s.Tag == "" {
		return fmt.Errorf("hashtag is required")
	}

	return nil
}

func (s *SourceTag) Stream(ctx context.Context, feed chan<- types.Activity, errs chan<- error) {
	client := mastodon.NewClient(&mastodon.Config{
		Server:       s.InstanceURL,
		ClientID:     "pulse-feed-aggregation",
		ClientSecret: "pulse-feed-aggregation",
	})

	limit := 15
	posts, err := s.fetchHashtagPosts(client, limit)
	if err != nil {
		errs <- fmt.Errorf("failed to fetch posts: %w", err)
		return
	}

	for _, post := range posts {
		feed <- post
	}
}

func (s *SourceTag) fetchHashtagPosts(client *mastodon.Client, limit int) ([]*Post, error) {
	statuses, err := client.GetTimelineHashtag(context.Background(), s.Tag, false, &mastodon.Pagination{
		Limit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get hashtag timeline: %w", err)
	}

	posts := make([]*Post, len(statuses))
	for i, status := range statuses {
		posts[i] = &Post{Status: status, SourceTyp: s.Type(), SourceID: s.UID()}
	}

	return posts, nil
}

func (s *SourceTag) MarshalJSON() ([]byte, error) {
	type Alias SourceTag
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  s.Type(),
	})
}

func (s *SourceTag) UnmarshalJSON(data []byte) error {
	type Alias SourceTag
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
