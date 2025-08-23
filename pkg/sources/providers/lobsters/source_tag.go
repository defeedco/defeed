package lobsters

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/lib"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/rs/zerolog"
)

const TypeLobstersTag = "lobsters:tag"

type SourceTag struct {
	InstanceURL string `json:"instanceUrl" validate:"required,url"`
	CustomURL   string `json:"customUrl" validate:"omitempty,url"`
	Tag         string `json:"tag" validate:"required"`
	client      *LobstersClient
	logger      *zerolog.Logger
}

func NewSourceTag() *SourceTag {
	return &SourceTag{
		InstanceURL: "https://lobste.rs",
	}
}

func (s *SourceTag) UID() string {
	return fmt.Sprintf("%s:%s:%s", s.Type(), strings.ReplaceAll(lib.StripURL(s.InstanceURL), "/", ":"), s.Tag)
}

func (s *SourceTag) Name() string {
	return fmt.Sprintf("#%s Tag", s.Tag)
}

func (s *SourceTag) Description() string {
	instanceName, err := lib.StripURLHost(s.InstanceURL)
	if err != nil {
		return fmt.Sprintf("Stories tagged with #%s from %s", s.Tag, instanceName)
	}
	return fmt.Sprintf("Stories tagged with #%s from %s", s.Tag, instanceName)
}

func (s *SourceTag) URL() string {
	return fmt.Sprintf("https://lobste.rs/t/%s", s.Tag)
}

func (s *SourceTag) Type() string {
	return TypeLobstersTag
}

func (s *SourceTag) Validate() []error {
	return lib.ValidateStruct(s)
}

func (s *SourceTag) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
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

func (s *SourceTag) fetchAndSendNewStories(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
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

	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	for _, story := range stories {
		post := &Post{Post: story, SourceTyp: s.Type(), SourceID: s.UID()}
		if since == nil || post.CreatedAt().After(sinceTime) {
			feed <- post
		}
	}
}

func (s *SourceTag) Initialize(logger *zerolog.Logger) error {
	s.client = NewLobstersClient(s.InstanceURL)
	s.logger = logger
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
