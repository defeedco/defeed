package lobsters

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/defeedco/defeed/pkg/lib"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	sourcetypes "github.com/defeedco/defeed/pkg/sources/types"
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

func (s *SourceTag) UID() activitytypes.TypedUID {
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

func (s *SourceTag) Icon() string {
	return "https://lobste.rs/favicon.ico"
}

func (s *SourceTag) Topics() []sourcetypes.TopicTag {
	base := []sourcetypes.TopicTag{sourcetypes.TopicDevTools, sourcetypes.TopicOpenSource}
	switch s.Tag {
	case "ai", "ml", "compsci":
		base = append(base, sourcetypes.TopicAIResearch)
	case "web", "performance":
		base = append(base, sourcetypes.TopicWebPerformance)
	case "kubernetes", "cloud":
		base = append(base, sourcetypes.TopicCloudInfrastructure)
	case "security":
		base = append(base, sourcetypes.TopicSecurityEngineering)
	case "linux", "rust", "wasm", "compilers", "plt":
		base = append(base, sourcetypes.TopicSystemsProgramming)
	case "databases":
		base = append(base, sourcetypes.TopicDatabases)
	}
	return base
}

func (s *SourceTag) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	s.fetchAndSendNewStories(ctx, since, feed, errs)
}

func (s *SourceTag) fetchAndSendNewStories(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
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
		post := &Post{
			Post:      story,
			SourceTyp: TypeLobstersTag,
			SourceIDs: []activitytypes.TypedUID{s.UID()},
		}
		if since == nil || post.CreatedAt().After(sinceTime) {
			feed <- post
		}
	}
}

func (s *SourceTag) Initialize(logger *zerolog.Logger, config *sourcetypes.ProviderConfig) error {
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
