package lobsters

import (
	"context"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/common"
)

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

func (s *SourceFeed) Initialize() error {
	if s.FeedName != "hottest" && s.FeedName != "newest" {
		return fmt.Errorf("feed name must be one of: 'hottest', 'newest'")
	}

	s.client = NewLobstersClient(s.InstanceURL)

	return nil
}

func (s *SourceFeed) Stream(ctx context.Context, feed chan<- common.Activity, errs chan<- error) {
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
		feed <- &lobstersPost{raw: story, sourceUID: s.UID()}
	}

}
