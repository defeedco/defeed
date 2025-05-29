package lobsters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
)

const TypeLobstersFeed = "lobsters-feed"

type SourceFeed struct {
	InstanceURL string `json:"instance_url"`
	CustomURL   string `json:"custom_url"`
	FeedName    string `json:"feed"`
	client      *LobstersClient
}

func NewSourceFeed() *SourceFeed {
	return &SourceFeed{
		InstanceURL: "https://lobste.rs",
	}
}

func (s *SourceFeed) UID() string {
	return fmt.Sprintf("lobsters-feed/%s/%s", s.InstanceURL, s.FeedName)
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

func (s *SourceFeed) Initialize() error {
	if s.FeedName != "hottest" && s.FeedName != "newest" {
		return fmt.Errorf("feed name must be one of: 'hottest', 'newest'")
	}

	s.client = NewLobstersClient(s.InstanceURL)

	return nil
}

func (s *SourceFeed) Stream(ctx context.Context, feed chan<- types.Activity, errs chan<- error) {
	var stories []*Story
	var err error

	if s.CustomURL != "" {
		stories, err = s.client.GetStoriesFromCustomURL(ctx, s.CustomURL)
	} else {
		stories, err = s.client.GetStoriesByFeed(ctx, s.FeedName)
	}

	if err != nil {
		errs <- err
		return
	}

	for _, story := range stories {
		feed <- &Post{Post: story, SourceTyp: s.Type(), SourceID: s.UID()}
	}

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
