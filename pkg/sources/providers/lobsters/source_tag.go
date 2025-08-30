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

const TypeLobstersTag = "lobsterstag"

type SourceTag struct {
	InstanceURL    string `json:"instanceUrl" validate:"required,url"`
	Tag            string `json:"tag" validate:"required"`
	TagDescription string `json:"tagDescription"`
	client         *LobstersClient
	logger         *zerolog.Logger
}

func NewSourceTag() *SourceTag {
	return &SourceTag{
		InstanceURL: "https://lobste.rs",
	}
}

func (s *SourceTag) UID() types.TypedUID {
	return lib.NewTypedUID(TypeLobstersTag, lib.StripURL(s.InstanceURL), s.Tag)
}

func (s *SourceTag) Name() string {
	return fmt.Sprintf("Lobsters #%s", s.Tag)
}

func (s *SourceTag) Description() string {
	if s.TagDescription != "" {
		return s.TagDescription
	}

	instanceName, err := lib.StripURLHost(s.InstanceURL)
	if err != nil {
		return fmt.Sprintf("Stories tagged with #%s from %s", s.Tag, instanceName)
	}
	return fmt.Sprintf("Stories tagged with #%s from %s", s.Tag, instanceName)
}

func (s *SourceTag) URL() string {
	return fmt.Sprintf("https://lobste.rs/t/%s", s.Tag)
}

func (s *SourceTag) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	s.fetchAndSendNewStories(ctx, since, feed, errs)
}

func (s *SourceTag) fetchAndSendNewStories(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	stories, err := s.client.GetStoriesByTag(ctx, s.Tag)

	if err != nil {
		errs <- err
		return
	}

	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	for _, story := range stories {
		post := &Post{Post: story, SourceTyp: TypeLobstersTag, SourceID: s.UID()}
		if since == nil || post.CreatedAt().After(sinceTime) {
			feed <- post
		}
	}
}

func (s *SourceTag) Initialize(logger *zerolog.Logger) error {
	if err := lib.ValidateStruct(s); err != nil {
		return err
	}

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
		Type:  TypeLobstersTag,
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
