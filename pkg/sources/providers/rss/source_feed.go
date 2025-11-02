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

	"github.com/defeedco/defeed/pkg/lib"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	sourcetypes "github.com/defeedco/defeed/pkg/sources/types"
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
	title       string
	description string
	topics      []sourcetypes.TopicTag
	FeedURL     string            `json:"url" validate:"required,url"`
	Headers     map[string]string `json:"headers"`
	IconURL     string            `json:"icon_url"`
	logger      *zerolog.Logger
}

func NewSourceFeed() *SourceFeed {
	return &SourceFeed{}
}

func (s *SourceFeed) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(TypeRSSFeed, lib.StripURL(s.FeedURL))
}

func (s *SourceFeed) Name() string {
	if s.title != "" {
		return s.title
	}

	hostName, err := lib.StripURLHost(s.FeedURL)
	if err == nil {
		return fmt.Sprintf("%s RSS Feed", lib.Capitalize(hostName))
	}

	return "RSS Feed"
}

func (s *SourceFeed) Description() string {
	if s.description != "" {
		return s.description
	}
	return fmt.Sprintf("Updates from %s", lib.StripURL(s.FeedURL))
}

func (s *SourceFeed) URL() string {
	return s.FeedURL
}

func (s *SourceFeed) Icon() string {
	return s.IconURL
}

func (s *SourceFeed) Topics() []sourcetypes.TopicTag {
	return s.topics
}

func (s *SourceFeed) getWebsiteURL() string {
	// Try to extract the website URL from the feed URL
	// For example, if feed URL is https://example.com/feed.xml,
	// the website URL would be https://example.com
	parsedURL, err := url.Parse(s.FeedURL)
	if err != nil {
		return ""
	}

	return parsedURL.Scheme + "://" + parsedURL.Host
}

func (s *SourceFeed) Initialize(logger *zerolog.Logger, config *sourcetypes.ProviderConfig) error {
	if err := lib.ValidateStruct(s); err != nil {
		return err
	}

	s.logger = logger

	return nil
}

func (s *SourceFeed) fetchIcon(ctx context.Context, logger *zerolog.Logger) error {
	// If favicon URL is already set (from OPML), use it
	if s.IconURL != "" {
		return nil
	}

	// Otherwise, try to fetch it automatically
	websiteURL := s.getWebsiteURL()
	if websiteURL != "" {
		resp, err := lib.FetchURL(ctx, logger, websiteURL)
		if err != nil {
			return fmt.Errorf("fetch url: %w", err)
		}
		defer resp.Body.Close()

		s.IconURL, err = lib.FaviconFromHTTPResponse(ctx, logger, resp)
		if err != nil {
			return fmt.Errorf("favicon from http response: %w", err)
		}
	}
	return nil
}

func (s *SourceFeed) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	s.fetchAndSendNewItems(ctx, since, feed, errs)
}

func (s *SourceFeed) fetchAndSendNewItems(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	parser := gofeed.NewParser()
	parser.UserAgent = lib.DefeedUserAgentString

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
		errs <- fmt.Errorf("fetch rss feed: %w", err)
		return
	}

	if rssFeed == nil {
		errs <- fmt.Errorf("feed is nil")
		return
	}

	if len(rssFeed.Items) == 0 {
		return
	}

	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	for _, item := range rssFeed.Items {
		if item.PublishedParsed == nil {
			s.logger.Warn().Msgf("skipping item with no published date: %+v", item)
			continue
		}
		// Skip items that are older or haven't been updated since the last seen activity
		if item.PublishedParsed.Before(sinceTime) &&
			(item.UpdatedParsed == nil || item.UpdatedParsed.Before(sinceTime)) {
			continue
		}

		feedItem := &FeedItem{
			Item:         item,
			FeedURL:      s.FeedURL,
			ThumbnailURL: "",
			SourceTyp:    TypeRSSFeed,
			SourceIDs:    []activitytypes.TypedUID{s.UID()},
		}

		if item.Image != nil && item.Image.URL != "" {
			feedItem.ThumbnailURL = item.Image.URL
		} else {
			thumbnailURL, err := lib.FetchThumbnailFromURL(ctx, s.logger, item.Link)
			if err == nil {
				feedItem.ThumbnailURL = thumbnailURL
			} else {
				s.logger.Warn().Err(err).
					Str("link", item.Link).
					Msgf("fetch rss item thumbnail")
			}
		}

		feed <- feedItem
	}
}

type FeedItem struct {
	Item         *gofeed.Item             `json:"item"`
	FeedURL      string                   `json:"feed_url"`
	ThumbnailURL string                   `json:"thumbnail_url"`
	SourceIDs    []activitytypes.TypedUID `json:"source_ids"`
	SourceTyp    string                   `json:"source_type"`
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
		SourceIDs []*lib.TypedUID `json:"source_ids"`
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.SourceIDs) == 0 {
		return fmt.Errorf("source_ids is required")
	}

	e.SourceIDs = make([]activitytypes.TypedUID, len(aux.SourceIDs))
	for i, uid := range aux.SourceIDs {
		e.SourceIDs[i] = uid
	}

	return nil
}

func (e *FeedItem) UID() activitytypes.TypedUID {
	id := e.Item.GUID
	if id == "" {
		id = lib.StripURL(e.URL())
	}
	return lib.NewTypedUID(e.SourceTyp, id)
}

func (e *FeedItem) SourceUIDs() []activitytypes.TypedUID {
	return e.SourceIDs
}

func (e *FeedItem) Title() string {
	if e.Item.Title != "" {
		return html.UnescapeString(e.Item.Title)
	}
	return shortenFeedDescriptionLen(e.Item.Description, 100)
}

func (e *FeedItem) Body() string {
	var raw string
	if e.Item.Content != "" {
		raw = e.Item.Content
	}
	if e.Item.Description != "" {
		raw = e.Item.Description
	}
	if raw != "" {
		text, err := lib.HTMLToText(raw)
		if err == nil {
			return text
		}
	}
	return raw
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
	if e.ThumbnailURL != "" {
		return e.ThumbnailURL
	}
	if e.Item.Image != nil && e.Item.Image.URL != "" {
		return e.Item.Image.URL
	}
	if thumbURL := findThumbnailInItemExtensions(e.Item); thumbURL != "" {
		return thumbURL
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

func (e *FeedItem) UpvotesCount() int {
	return -1
}

func (e *FeedItem) DownvotesCount() int {
	return -1
}

func (e *FeedItem) CommentsCount() int {
	return -1
}

func (e *FeedItem) AmplificationCount() int {
	return -1
}

func (e *FeedItem) SocialScore() float64 {
	return -1
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
				if attrURL, ok := ext.Attrs["url"]; ok {
					return attrURL
				}
			}

			if ext.Children != nil {
				if childURL := recursiveFindThumbnailInExtensions(ext.Children); childURL != "" {
					return childURL
				}
			}
		}
	}

	return ""
}

var htmlTagsWithAttributesPattern = regexp.MustCompile(`</?[a-zA-Z0-9-]+ *(?:[a-zA-Z-]+=["'].*?["'] ?)* */?>`)
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
