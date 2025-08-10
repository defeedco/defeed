package lobsters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
)

const TypeLobstersTag = "lobsters-tag"

type SourceTag struct {
	InstanceURL string `json:"instanceUrl"`
	CustomURL   string `json:"customUrl"`
	Tag         string `json:"tag"`
	client      *LobstersClient
}

func NewSourceTag() *SourceTag {
	return &SourceTag{
		InstanceURL: "https://lobste.rs",
	}
}

func (s *SourceTag) UID() string {
	return fmt.Sprintf("%s/%s/%s", s.Type(), s.InstanceURL, s.Tag)
}

func (s *SourceTag) Name() string {
	return fmt.Sprintf("Lobsters (#%s)", s.Tag)
}

func (s *SourceTag) URL() string {
	return fmt.Sprintf("https://lobste.rs/t/%s", s.Tag)
}

func (s *SourceTag) Type() string {
	return TypeLobstersTag
}

func (s *SourceTag) Stream(ctx context.Context, feed chan<- types.Activity, errs chan<- error) {
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
		feed <- &Post{Post: story, SourceTyp: s.Type(), SourceID: s.UID()}
	}
}

func (s *SourceTag) Initialize() error {
	if s.Tag == "" {
		return fmt.Errorf("tag is required")
	}

	s.client = NewLobstersClient(s.InstanceURL)

	return nil
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
