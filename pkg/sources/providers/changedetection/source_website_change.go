package changedetection

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/rs/zerolog"
)

const TypeChangedetectionWebsite = "changedetection:website"

type SourceWebsiteChange struct {
	WatchUUID   string `json:"watch" validate:"required"`
	InstanceURL string `json:"instanceUrl" validate:"omitempty,url"`
	Token       string `json:"token"`
	Limit       int    `json:"limit"`
	logger      *zerolog.Logger
}

func NewSourceWebsiteChange() *SourceWebsiteChange {
	return &SourceWebsiteChange{}
}

func (s *SourceWebsiteChange) UID() lib.TypedUID {
	return lib.NewTypedUID(TypeChangedetectionWebsite, lib.StripURL(s.InstanceURL), s.WatchUUID)
}

func (s *SourceWebsiteChange) Name() string {
	if s.WatchUUID != "" {
		return fmt.Sprintf("Website Change Monitor (%s)", s.WatchUUID[:8])
	}
	return "Website Change Monitor"
}

func (s *SourceWebsiteChange) Description() string {
	instanceName, err := lib.StripURLHost(s.InstanceURL)
	if err != nil {
		return "Website Change Monitor"
	}
	return fmt.Sprintf("Monitor website changes using %s", instanceName)
}

func (s *SourceWebsiteChange) URL() string {
	return s.InstanceURL
}

func (s *SourceWebsiteChange) Validate() []error { return lib.ValidateStruct(s) }

func (s *SourceWebsiteChange) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	s.fetchAndSendNewChanges(ctx, since, feed, errs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchAndSendNewChanges(ctx, since, feed, errs)
		}
	}
}

func (s *SourceWebsiteChange) fetchAndSendNewChanges(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	change, err := s.fetchWatchFromChangeDetection(ctx)
	if err != nil {
		errs <- err
		return
	}

	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	if since == nil || change.CreatedAt().After(sinceTime) {
		feed <- change
	}
}

func (s *SourceWebsiteChange) Initialize(logger *zerolog.Logger) error {
	if s.Limit <= 0 {
		s.Limit = 10
	}

	if s.InstanceURL == "" {
		s.InstanceURL = "https://www.changedetection.io"
	}

	s.logger = logger

	return nil
}

func (s *SourceWebsiteChange) MarshalJSON() ([]byte, error) {
	type Alias SourceWebsiteChange
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  TypeChangedetectionWebsite,
	})
}

func (s *SourceWebsiteChange) UnmarshalJSON(data []byte) error {
	type Alias SourceWebsiteChange
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

// TODO: This source hasn't been tested or fully implemented
type WebsiteChange struct {
	title        string
	url          string
	lastChanged  time.Time
	diffURL      string
	previousHash string
	sourceUID    lib.TypedUID
}

func NewWebsiteChange() *WebsiteChange {
	return &WebsiteChange{}
}

func (c *WebsiteChange) SourceType() string {
	return TypeChangedetectionWebsite
}

func (c *WebsiteChange) MarshalJSON() ([]byte, error) {
	type Alias WebsiteChange
	return json.Marshal(&struct {
		*Alias
		LastChanged string `json:"last_changed"`
	}{
		Alias:       (*Alias)(c),
		LastChanged: c.lastChanged.Format(time.RFC3339),
	})
}

func (c *WebsiteChange) UnmarshalJSON(data []byte) error {
	type Alias WebsiteChange
	aux := &struct {
		*Alias
		LastChanged string `json:"last_changed"`
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	c.lastChanged = lib.ParseRFC3339Time(aux.LastChanged)
	return nil
}

func (c *WebsiteChange) SourceUID() lib.TypedUID {
	return c.sourceUID
}

func (c *WebsiteChange) UID() lib.TypedUID {
	return lib.NewTypedUID(TypeChangedetectionWebsite, lib.StripURL(c.url), fmt.Sprintf("%d", c.lastChanged.Unix()))
}

func (c *WebsiteChange) Title() string {
	return c.title
}

func (c *WebsiteChange) Body() string {
	return ""
}

func (c *WebsiteChange) URL() string {
	return c.url
}

func (c *WebsiteChange) ImageURL() string {
	// TODO(pulse): Use website favicon
	return ""
}

func (c *WebsiteChange) CreatedAt() time.Time {
	return c.lastChanged
}

type changeDetectionWatchResponseJson struct {
	Title        string `json:"title"`
	URL          string `json:"url"`
	LastChanged  string `json:"last_changed"`
	DiffURL      string `json:"diff_url"`
	PreviousHash string `json:"previous_hash"`
}

func (s *SourceWebsiteChange) fetchWatchFromChangeDetection(ctx context.Context) (*WebsiteChange, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/api/v1/watch/%s", s.InstanceURL, s.WatchUUID),
		nil,
	)
	if err != nil {
		return nil, err
	}

	if s.Token != "" {
		req.Header.Add("X-API-Key", s.Token)
	}

	response, err := lib.DecodeJSONFromRequest[changeDetectionWatchResponseJson](lib.DefaultHTTPClient, req)
	if err != nil {
		return nil, err
	}

	return &WebsiteChange{
		title:       response.Title,
		url:         response.URL,
		lastChanged: lib.ParseRFC3339Time(response.LastChanged),
		diffURL:     response.DiffURL,
		// TODO: is there a current hash as well that we can use as a UID?
		previousHash: response.PreviousHash,
		sourceUID:    s.UID(),
	}, nil
}
