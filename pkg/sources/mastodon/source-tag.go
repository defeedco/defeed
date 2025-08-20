package mastodon

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/glanceapp/glance/pkg/utils"

	"github.com/mattn/go-mastodon"
)

const TypeMastodonTag = "mastodon-tag"

type SourceTag struct {
	InstanceURL string `json:"instanceUrl" validate:"required,url"`
	Tag         string `json:"tag" validate:"required"`
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

func (s *SourceTag) Validate() []error { return utils.ValidateStruct(s) }

func (s *SourceTag) Initialize() error { return nil }

func (s *SourceTag) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	s.fetchAndSendNewPosts(ctx, since, feed, errs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchAndSendNewPosts(ctx, since, feed, errs)
		}
	}
}

func (s *SourceTag) fetchAndSendNewPosts(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	client := mastodon.NewClient(&mastodon.Config{
		Server:       s.InstanceURL,
		ClientID:     "pulse-feed-aggregation",
		ClientSecret: "pulse-feed-aggregation",
	})

	limit := 15
	posts, err := s.fetchHashtagPosts(ctx, client, limit)
	if err != nil {
		errs <- fmt.Errorf("failed to fetch posts: %w", err)
		return
	}

	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	for _, post := range posts {
		if since == nil || post.CreatedAt().After(sinceTime) {
			feed <- post
		}
	}
}

func (s *SourceTag) fetchHashtagPosts(ctx context.Context, client *mastodon.Client, limit int) ([]*Post, error) {
	statuses, err := client.GetTimelineHashtag(ctx, s.Tag, false, &mastodon.Pagination{
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
