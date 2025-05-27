package changedetection

import (
	"context"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/common"
	"net/http"
	"sort"
	"time"
)

type SourceWebsiteChange struct {
	WatchUUID   string `json:"watch"`
	InstanceURL string `json:"instance_url"`
	Token       string `json:"token"`
	Limit       int    `json:"limit"`
}

func NewSourceWebsiteChange() *SourceWebsiteChange {
	return &SourceWebsiteChange{}
}

func (s *SourceWebsiteChange) UID() string {
	return fmt.Sprintf("change-detection/%s", s.WatchUUID)
}

func (s *SourceWebsiteChange) Name() string {
	return "Change Detection"
}

func (s *SourceWebsiteChange) URL() string {
	return s.InstanceURL
}

func (s *SourceWebsiteChange) Stream(ctx context.Context, feed chan<- common.Activity, errs chan<- error) {
	initial, err := s.fetchWatchFromChangeDetection()

	if err != nil {
		errs <- err
		return
	}

	feed <- initial
}

func (s *SourceWebsiteChange) Initialize() error {
	if s.Limit <= 0 {
		s.Limit = 10
	}

	if s.InstanceURL == "" {
		s.InstanceURL = "https://www.changedetection.io"
	}

	return nil
}

type websiteChange struct {
	title        string
	url          string
	lastChanged  time.Time
	diffURL      string
	previousHash string
	sourceUID    string
}

func (c websiteChange) SourceUID() string {
	return c.sourceUID
}

func (c websiteChange) UID() string {
	return fmt.Sprintf("%s-%d", c.url, c.lastChanged.Unix())
}

func (c websiteChange) Title() string {
	return c.title
}

func (c websiteChange) Body() string {
	return ""
}

func (c websiteChange) URL() string {
	return c.url
}

func (c websiteChange) ImageURL() string {
	// TODO(pulse): Use website favicon
	return ""
}

func (c websiteChange) CreatedAt() time.Time {
	return c.lastChanged
}

type changeDetectionWatchList []websiteChange

func (r changeDetectionWatchList) sortByNewest() changeDetectionWatchList {
	sort.Slice(r, func(i, j int) bool {
		return r[i].lastChanged.After(r[j].lastChanged)
	})

	return r
}

type changeDetectionWatchResponseJson struct {
	Title        string `json:"title"`
	URL          string `json:"url"`
	LastChanged  string `json:"last_changed"`
	DiffURL      string `json:"diff_url"`
	PreviousHash string `json:"previous_hash"`
}

func (s *SourceWebsiteChange) fetchWatchFromChangeDetection() (websiteChange, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/api/v1/watch/%s", s.InstanceURL, s.WatchUUID),
		nil,
	)
	if err != nil {
		return websiteChange{}, err
	}

	if s.Token != "" {
		req.Header.Add("X-API-Key", s.Token)
	}

	response, err := common.DecodeJSONFromRequest[changeDetectionWatchResponseJson](common.DefaultHTTPClient, req)
	if err != nil {
		return websiteChange{}, err
	}

	return websiteChange{
		title:        response.Title,
		url:          response.URL,
		lastChanged:  common.ParseRFC3339Time(response.LastChanged),
		diffURL:      response.DiffURL,
		previousHash: response.PreviousHash,
		sourceUID:    s.UID(),
	}, nil
}
