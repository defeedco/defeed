package mastodon

import (
	"context"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/common"
	"github.com/mattn/go-mastodon"
)

type SourceTag struct {
	InstanceURL string `json:"instance_url"`
	Tag         string `json:"tag"`
}

func NewSourceTag() *SourceTag {
	return &SourceTag{
		InstanceURL: "https://mastodon.social",
	}
}

func (s *SourceTag) UID() string {
	return fmt.Sprintf("mastodon/%s/%s", s.InstanceURL, s.Tag)
}

func (s *SourceTag) Name() string {
	return fmt.Sprintf("Mastodon (%s)", s.Tag)
}

func (s *SourceTag) URL() string {
	return fmt.Sprintf("%s/tags/%s", s.InstanceURL, s.Tag)
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

func (s *SourceTag) Stream(ctx context.Context, feed chan<- common.Activity, errs chan<- error) {
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

func (s *SourceTag) fetchHashtagPosts(client *mastodon.Client, limit int) ([]*mastodonPost, error) {
	statuses, err := client.GetTimelineHashtag(context.Background(), s.Tag, false, &mastodon.Pagination{
		Limit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get hashtag timeline: %w", err)
	}

	posts := make([]*mastodonPost, len(statuses))
	for i, status := range statuses {
		posts[i] = &mastodonPost{raw: status, sourceUID: s.UID()}
	}

	return posts, nil
}
