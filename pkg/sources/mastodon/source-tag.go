package mastodon

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/glanceapp/glance/pkg/utils"

	"github.com/mattn/go-mastodon"
	"github.com/rs/zerolog"
)

const TypeMastodonTag = "mastodon-tag"

type SourceTag struct {
	InstanceURL string `json:"instanceUrl" validate:"required,url"`
	Tag         string `json:"tag" validate:"required"`
	client      *mastodon.Client
	logger      *zerolog.Logger
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

func (s *SourceTag) Initialize(logger *zerolog.Logger) error {
	s.client = mastodon.NewClient(&mastodon.Config{
		Server:       s.InstanceURL,
		ClientID:     "pulse-feed-aggregation",
		ClientSecret: "pulse-feed-aggregation",
	})

	s.logger = logger

	return nil
}

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
	posts, err := s.fetchHashtagPosts(ctx, since)
	if err != nil {
		errs <- fmt.Errorf("fetch posts: %w", err)
		return
	}

	for _, post := range posts {
		feed <- post
	}
}

func (s *SourceTag) fetchHashtagPosts(ctx context.Context, since types.Activity) ([]*Post, error) {
	var sinceID mastodon.ID
	if since != nil {
		sincePost := since.(*Post)
		sinceID = sincePost.Status.ID
	} else {
		// If this is the first time we're fetching posts,
		// only fetch the last few posts to avoid fetching all historic posts.
		latestPosts, err := s.fetchLatestPosts(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetch latest post: %w", err)
		}
		return latestPosts, nil
	}

	posts := make([]*Post, 0)
outer:
	for {
		tagLogger := s.logger.With().
			Str("tag", s.Tag).
			Str("since_id", string(sinceID)).
			Logger()

		tagLogger.Debug().Msg("Fetching hashtag timeline")
		statuses, err := s.client.GetTimelineHashtag(ctx, s.Tag, false, &mastodon.Pagination{
			Limit:   int64(15),
			SinceID: sinceID,
		})
		if err != nil {
			return nil, fmt.Errorf("get hashtag timeline: %w", err)
		}

		tagLogger.Debug().Int("count", len(statuses)).Msg("Fetched hashtag timeline")

		if len(statuses) == 0 {
			break outer
		}

		for _, status := range statuses {
			posts = append(posts, &Post{
				Status:    status,
				SourceTyp: s.Type(),
				SourceID:  s.UID(),
			})
		}

		sinceID = statuses[len(statuses)-1].ID
	}

	return posts, nil
}

func (s *SourceTag) fetchLatestPosts(ctx context.Context) ([]*Post, error) {
	tagLogger := s.logger.With().
		Str("tag", s.Tag).
		Logger()

	tagLogger.Debug().Msg("Fetching latest post from hashtag timeline")

	statuses, err := s.client.GetTimelineHashtag(ctx, s.Tag, false, &mastodon.Pagination{
		Limit: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("get hashtag timeline: %w", err)
	}

	if len(statuses) == 0 {
		tagLogger.Debug().Msg("No posts found in hashtag timeline")
		return nil, nil
	}

	posts := make([]*Post, 0)
	for _, status := range statuses {
		posts = append(posts, &Post{
			Status:    status,
			SourceTyp: s.Type(),
			SourceID:  s.UID(),
		})
	}

	tagLogger.Debug().Int("count", len(posts)).Msg("Fetched latest posts from hashtag timeline")

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
