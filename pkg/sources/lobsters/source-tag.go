package lobsters

import (
	"context"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/common"
)

type SourceTag struct {
	InstanceURL string `json:"instance_url"`
	CustomURL   string `json:"custom_url"`
	Tag         string `json:"tag"`
	client      *LobstersClient
}

func NewSourceTag() *SourceTag {
	return &SourceTag{
		InstanceURL: "https://lobste.rs",
	}
}

func (s *SourceTag) UID() string {
	return fmt.Sprintf("lobsters-tag/%s/%s", s.InstanceURL, s.Tag)
}

func (s *SourceTag) Name() string {
	return fmt.Sprintf("Lobsters (#%s)", s.Tag)
}

func (s *SourceTag) URL() string {
	return fmt.Sprintf("https://lobste.rs/t/%s", s.Tag)
}

func (s *SourceTag) Stream(ctx context.Context, feed chan<- common.Activity, errs chan<- error) {
	var stories []*Story
	var err error

	if s.CustomURL != "" {
		stories, err = s.client.GetStoriesFromCustomURL(ctx, s.CustomURL)
	} else {
		stories, err = s.client.GetStoriesByTag(ctx, s.Tag)
	}

	if err != nil {
		errs <- err
		return
	}

	for _, story := range stories {
		feed <- &lobstersPost{raw: story, sourceUID: s.UID()}
	}
}

func (s *SourceTag) Initialize() error {
	if s.Tag == "" {
		return fmt.Errorf("tag is required")
	}

	s.client = NewLobstersClient(s.InstanceURL)

	return nil
}
