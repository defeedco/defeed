package rss

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/mmcdole/gofeed"
	gofeedext "github.com/mmcdole/gofeed/extensions"
	"github.com/rs/zerolog"
)

const TypeRSSFeed = "rssfeed"

type customTransport struct {
	headers map[string]string
	base    http.RoundTripper
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}
	return t.base.RoundTrip(req)
}

type SourceFeed struct {
	Title   string
	Tags    []string
	FeedURL string            `json:"url" validate:"required,url"`
	Headers map[string]string `json:"headers"`
	logger  *zerolog.Logger
}

func NewSourceFeed() *SourceFeed {
	return &SourceFeed{}
}

func (s *SourceFeed) UID() lib.TypedUID {
	return lib.NewSimpleTypedUID(TypeRSSFeed, lib.StripURL(s.FeedURL))
}

func (s *SourceFeed) Name() string {
	if s.Title != "" {
		return s.Title
	}

	hostName, err := lib.StripURLHost(s.FeedURL)
	if err == nil {
		return fmt.Sprintf("%s RSS Feed", lib.Capitalize(hostName))
	}

	return "RSS Feed"
}

func (s *SourceFeed) Description() string {
	return fmt.Sprintf("Updates from %s", lib.StripURL(s.FeedURL))
}

func (s *SourceFeed) URL() string {
	return s.FeedURL
}

func (s *SourceFeed) Validate() []error { return lib.ValidateStruct(s) }

func (s *SourceFeed) Initialize(logger *zerolog.Logger) error {
	s.logger = logger
	return nil
}

func (s *SourceFeed) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	s.fetchAndSendNewItems(ctx, since, feed, errs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchAndSendNewItems(ctx, since, feed, errs)
		}
	}
}

func (s *SourceFeed) fetchAndSendNewItems(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	parser := gofeed.NewParser()
	parser.UserAgent = lib.PulseUserAgentString

	if s.Headers != nil {
		parser.Client = &http.Client{
			Transport: &customTransport{
				headers: s.Headers,
				base:    http.DefaultTransport,
			},
		}
	}

	rssFeed, err := parser.ParseURLWithContext(s.FeedURL, ctx)
	if err != nil {
		errs <- fmt.Errorf("failed to parse RSS feed: %w", err)
		return
	}

	if rssFeed == nil {
		errs <- fmt.Errorf("feed is nil")
		return
	}

	if len(rssFeed.Items) == 0 {
		errs <- fmt.Errorf("feed has no items")
		return
	}

	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	for _, item := range rssFeed.Items {
		if since != nil && item.PublishedParsed.Before(sinceTime) {
			continue
		}

		feedItem := &FeedItem{
			Item:      item,
			FeedURL:   s.FeedURL,
			SourceTyp: TypeRSSFeed,
			SourceID:  s.UID(),
		}

		feed <- feedItem
	}
}

type FeedItem struct {
	Item      *gofeed.Item `json:"item"`
	FeedURL   string       `json:"feed_url"`
	SourceID  lib.TypedUID `json:"source_id"`
	SourceTyp string       `json:"source_type"`
}

func NewFeedItem() *FeedItem {
	return &FeedItem{}
}

func (e *FeedItem) MarshalJSON() ([]byte, error) {
	type Alias FeedItem
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	})
}

func (e *FeedItem) UnmarshalJSON(data []byte) error {
	type Alias FeedItem
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	return json.Unmarshal(data, &aux)
}

func (e *FeedItem) UID() lib.TypedUID {
	id := e.Item.GUID
	if id == "" {
		id = lib.StripURL(e.URL())
	}
	return lib.NewSimpleTypedUID(e.SourceTyp, id)
}

func (e *FeedItem) SourceUID() lib.TypedUID {
	return e.SourceID
}

func (e *FeedItem) Title() string {
	if e.Item.Title != "" {
		return html.UnescapeString(e.Item.Title)
	}
	return shortenFeedDescriptionLen(e.Item.Description, 100)
}

func (e *FeedItem) Body() string {
	if e.Item.Content != "" {
		return e.Item.Content
	}
	return e.Item.Description
}

func (e *FeedItem) URL() string {
	if strings.HasPrefix(e.Item.Link, "http://") || strings.HasPrefix(e.Item.Link, "https://") {
		return e.Item.Link
	}

	parsedUrl, err := url.Parse(e.FeedURL)
	if err == nil {
		link := e.Item.Link
		if !strings.HasPrefix(link, "/") {
			link = "/" + link
		}
		return parsedUrl.Scheme + "://" + parsedUrl.Host + link
	}
	return e.Item.Link
}

func (e *FeedItem) ImageURL() string {
	if e.Item.Image != nil && e.Item.Image.URL != "" {
		return e.Item.Image.URL
	}
	if url := findThumbnailInItemExtensions(e.Item); url != "" {
		return url
	}
	return ""
}

func (e *FeedItem) CreatedAt() time.Time {
	if e.Item.PublishedParsed != nil {
		return *e.Item.PublishedParsed
	}
	if e.Item.UpdatedParsed != nil {
		return *e.Item.UpdatedParsed
	}
	return time.Now()
}

func (e *FeedItem) Categories() []string {
	categories := make([]string, 0, len(e.Item.Categories))
	for _, category := range e.Item.Categories {
		if category != "" {
			categories = append(categories, category)
		}
	}
	return categories
}

func findThumbnailInItemExtensions(item *gofeed.Item) string {
	media, ok := item.Extensions["media"]

	if !ok {
		return ""
	}

	return recursiveFindThumbnailInExtensions(media)
}

func recursiveFindThumbnailInExtensions(extensions map[string][]gofeedext.Extension) string {
	for _, exts := range extensions {
		for _, ext := range exts {
			if ext.Name == "thumbnail" || ext.Name == "image" {
				if url, ok := ext.Attrs["url"]; ok {
					return url
				}
			}

			if ext.Children != nil {
				if url := recursiveFindThumbnailInExtensions(ext.Children); url != "" {
					return url
				}
			}
		}
	}

	return ""
}

var htmlTagsWithAttributesPattern = regexp.MustCompile(`<\/?[a-zA-Z0-9-]+ *(?:[a-zA-Z-]+=(?:"|').*?(?:"|') ?)* *\/?>`)
var sequentialWhitespacePattern = regexp.MustCompile(`\s+`)

func sanitizeFeedDescription(description string) string {
	if description == "" {
		return ""
	}

	description = strings.ReplaceAll(description, "\n", " ")
	description = htmlTagsWithAttributesPattern.ReplaceAllString(description, "")
	description = sequentialWhitespacePattern.ReplaceAllString(description, " ")
	description = strings.TrimSpace(description)
	description = html.UnescapeString(description)

	return description
}

func shortenFeedDescriptionLen(description string, maxLen int) string {
	description, _ = lib.LimitStringLength(description, 1000)
	description = sanitizeFeedDescription(description)
	description, limited := lib.LimitStringLength(description, maxLen)

	if limited {
		description += "…"
	}

	return description
}

func (s *SourceFeed) MarshalJSON() ([]byte, error) {
	type Alias SourceFeed
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  TypeRSSFeed,
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
