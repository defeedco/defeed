package lobsters

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/rs/zerolog"
)

const TypeLobstersFeed = "lobsters-feed"

type SourceFeed struct {
	InstanceURL string `json:"instanceUrl" validate:"required,url"`
	CustomURL   string `json:"customUrl" validate:"omitempty,url"`
	FeedName    string `json:"feed" validate:"required,oneof=hottest newest"`
	client      *LobstersClient
	logger      *zerolog.Logger
}

func NewSourceFeed() *SourceFeed {
	return &SourceFeed{
		InstanceURL: "https://lobste.rs",
	}
}

func (s *SourceFeed) UID() string {
	return fmt.Sprintf("%s/%s/%s", s.Type(), s.InstanceURL, s.FeedName)
}

func (s *SourceFeed) Name() string {
	return fmt.Sprintf("Lobsters (%s)", s.FeedName)
}

func (s *SourceFeed) URL() string {
	return fmt.Sprintf("https://lobste.rs/%s", s.FeedName)
}

func (s *SourceFeed) Type() string {
	return TypeLobstersFeed
}

func (s *SourceFeed) Validate() []error { return lib.ValidateStruct(s) }

func (s *SourceFeed) Initialize(logger *zerolog.Logger) error {
	s.client = NewLobstersClient(s.InstanceURL)
	s.logger = logger
	return nil
}

func (s *SourceFeed) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	s.fetchAndSendNewStories(ctx, since, feed, errs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchAndSendNewStories(ctx, since, feed, errs)
		}
	}
}

func (s *SourceFeed) fetchAndSendNewStories(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	feedLogger := s.logger.With().
		Str("feed", s.FeedName).
		Str("instance_url", s.InstanceURL).
		Logger()

	var stories []*Story
	var err error

	if s.CustomURL != "" {
		stories, err = s.client.GetStoriesFromCustomURL(ctx, s.CustomURL)
	} else {
		stories, err = s.client.GetStoriesByFeed(ctx, s.FeedName)
	}

	feedLogger.Debug().Int("count", len(stories)).Msg("Fetched stories")

	if err != nil {
		errs <- err
		return
	}

	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	for _, story := range stories {
		post, err := s.buildPost(ctx, story)
		if err != nil {
			errs <- err
			return
		}
		if since == nil || post.CreatedAt().After(sinceTime) {
			feed <- post
		}
	}
}

func (s *SourceFeed) buildPost(ctx context.Context, story *Story) (*Post, error) {
	post := &Post{Post: story, SourceTyp: s.Type(), SourceID: s.UID()}
	if story.URL != "" {
		externalContent, err := lib.FetchTextFromURL(ctx, story.URL)
		if err != nil {
			return nil, fmt.Errorf("fetch external content: %w", err)
		}
		post.ExternalContent = externalContent
	}
	return post, nil
}

func (s *SourceFeed) MarshalJSON() ([]byte, error) {
	type Alias SourceFeed
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  s.Type(),
	})
}

func (s *SourceFeed) UnmarshalJSON(data []byte) error {
	type Alias SourceFeed
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
