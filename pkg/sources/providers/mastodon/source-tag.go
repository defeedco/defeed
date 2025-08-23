package mastodon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/mattn/go-mastodon"
	"github.com/rs/zerolog"
)

const TypeMastodonTag = "mastodon:tag"

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
	urlID := s.InstanceURL
	// Normalize the URL for consistent UID format (cannot contain slashes)
	urlID = strings.TrimPrefix(urlID, "https://")
	urlID = strings.TrimPrefix(urlID, "http://")
	urlID = strings.TrimPrefix(urlID, "www.")
	urlID = strings.TrimSuffix(urlID, "/")
	urlID = strings.ReplaceAll(urlID, "/", ":")
	return fmt.Sprintf("%s:%s:%s", s.Type(), urlID, s.Tag)
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

func (s *SourceTag) Validate() []error { return lib.ValidateStruct(s) }

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

	s.fetchHashtagPosts(ctx, since, feed, errs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchHashtagPosts(ctx, since, feed, errs)
		}
	}
}

func (s *SourceTag) fetchHashtagPosts(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	var sinceID mastodon.ID
	if since != nil {
		sincePost := since.(*Post)
		sinceID = sincePost.Status.ID
	} else {
		// If this is the first time we're fetching posts,
		// only fetch the last few posts to avoid retrieving all historic posts.
		s.fetchLatestPosts(ctx, feed, errs)
		return
	}

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
			errs <- fmt.Errorf("get hashtag timeline: %w", err)
			return
		}

		tagLogger.Debug().Int("count", len(statuses)).Msg("Fetched hashtag timeline")

		if len(statuses) == 0 {
			break outer
		}

		for _, status := range statuses {
			post := &Post{
				Status:    status,
				SourceTyp: s.Type(),
				SourceID:  s.UID(),
			}
			feed <- post
		}

		sinceID = statuses[len(statuses)-1].ID
	}
}

func (s *SourceTag) fetchLatestPosts(ctx context.Context, feed chan<- types.Activity, errs chan<- error) {
	tagLogger := s.logger.With().
		Str("tag", s.Tag).
		Logger()

	tagLogger.Debug().Msg("Fetching latest post from hashtag timeline")

	statuses, err := s.client.GetTimelineHashtag(ctx, s.Tag, false, &mastodon.Pagination{
		Limit: 10,
	})
	if err != nil {
		errs <- fmt.Errorf("get hashtag timeline: %w", err)
		return
	}

	if len(statuses) == 0 {
		tagLogger.Debug().Msg("No posts found in hashtag timeline")
		return
	}

	for _, status := range statuses {
		post := &Post{
			Status:    status,
			SourceTyp: s.Type(),
			SourceID:  s.UID(),
		}
		feed <- post
	}

	tagLogger.Debug().Int("count", len(statuses)).Msg("Fetched latest posts from hashtag timeline")
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
