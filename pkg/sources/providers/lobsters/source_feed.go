package lobsters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	activitytypes "github.com/glanceapp/glance/pkg/sources/activities/types"
	sourcetypes "github.com/glanceapp/glance/pkg/sources/types"
	"github.com/rs/zerolog"
)

const TypeLobstersFeed = "lobstersfeed"

type SourceFeed struct {
	InstanceURL string `json:"instanceUrl" validate:"required,url"`
	FeedName    string `json:"feed" validate:"required,oneof=hottest newest"`
	client      *LobstersClient
	logger      *zerolog.Logger
}

func NewSourceFeed() *SourceFeed {
	return &SourceFeed{
		InstanceURL: "https://lobste.rs",
	}
}

func (s *SourceFeed) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(TypeLobstersFeed, lib.StripURL(s.InstanceURL), s.FeedName)
}

func (s *SourceFeed) Name() string {
	return fmt.Sprintf("%s on Lobsters", lib.Capitalize(s.FeedName))
}

func (s *SourceFeed) Description() string {
	instanceName, err := lib.StripURLHost(s.InstanceURL)
	if err != nil {
		return fmt.Sprintf("Stories from %s", instanceName)
	}

	switch s.FeedName {
	case "hottest":
		return fmt.Sprintf("Hottest stories from %s", instanceName)
	case "newest":
		return fmt.Sprintf("Newest stories from %s", instanceName)
	default:
		return fmt.Sprintf("%s stories from %s", lib.Capitalize(s.FeedName), instanceName)
	}
}

func (s *SourceFeed) URL() string {
	return fmt.Sprintf("https://lobste.rs/%s", s.FeedName)
}

func (s *SourceFeed) Icon() string {
	return "https://lobste.rs/favicon.ico"
}

func (s *SourceFeed) Topics() []sourcetypes.TopicTag {
	switch s.FeedName {
	case "hottest", "newest":
		return []sourcetypes.TopicTag{sourcetypes.TopicDevTools, sourcetypes.TopicOpenSource}
	}
	return []sourcetypes.TopicTag{sourcetypes.TopicDevTools}
}

func (s *SourceFeed) Initialize(logger *zerolog.Logger, config *sourcetypes.ProviderConfig) error {
	if err := lib.ValidateStruct(s); err != nil {
		return err
	}

	s.client = NewLobstersClient(s.InstanceURL)
	s.logger = logger
	return nil
}

func (s *SourceFeed) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	s.fetchAndSendNewStories(ctx, since, feed, errs)
}

func (s *SourceFeed) fetchAndSendNewStories(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	stories, err := s.client.GetStoriesByFeed(ctx, s.FeedName)
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
	post := &Post{Post: story, SourceTyp: TypeLobstersFeed, SourceID: s.UID()}
	if story.URL != "" {
		externalContent, err := lib.FetchTextFromURL(ctx, s.logger, story.URL)
		if err != nil && !errors.Is(err, lib.ErrUnsupportedContentType) {
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
		Type:  TypeLobstersFeed,
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
