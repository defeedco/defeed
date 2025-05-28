package rss

import (
	"context"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/common"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	gofeedext "github.com/mmcdole/gofeed/extensions"
)

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
	FeedURL string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func NewSourceFeed() *SourceFeed {
	return &SourceFeed{}
}

func (s *SourceFeed) UID() string {
	return fmt.Sprintf("rss/%s", s.FeedURL)
}

func (s *SourceFeed) Name() string {
	return fmt.Sprintf("RSS (%s)", s.FeedURL)
}

func (s *SourceFeed) URL() string {
	return s.FeedURL
}

func (s *SourceFeed) Initialize() error {
	if s.FeedURL == "" {
		return fmt.Errorf("URL is required")
	}

	return nil
}

func (s *SourceFeed) Stream(ctx context.Context, feed chan<- common.Activity, errs chan<- error) {
	parser := gofeed.NewParser()
	parser.UserAgent = common.PulseUserAgentString

	if s.Headers != nil {
		parser.Client = &http.Client{
			Transport: &customTransport{
				headers: s.Headers,
				base:    http.DefaultTransport,
			},
		}
	}

	rssFeed, err := parser.ParseURL(s.FeedURL)
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

	for _, item := range rssFeed.Items {
		feed <- &rssFeedItem{raw: item, feedLink: s.FeedURL, sourceUID: s.UID()}
	}
}

type rssFeedItem struct {
	raw       *gofeed.Item
	feedLink  string
	sourceUID string
}

func (i *rssFeedItem) UID() string {
	if i.raw.GUID != "" {
		return i.raw.GUID
	}
	return i.URL()
}

func (i *rssFeedItem) SourceUID() string {
	return i.sourceUID
}

func (i *rssFeedItem) Title() string {
	if i.raw.Title != "" {
		return html.UnescapeString(i.raw.Title)
	}
	return shortenFeedDescriptionLen(i.raw.Description, 100)
}

func (i *rssFeedItem) Body() string {
	if i.raw.Content != "" {
		return i.raw.Content
	}
	return i.raw.Description
}

func (i *rssFeedItem) URL() string {
	if strings.HasPrefix(i.raw.Link, "http://") || strings.HasPrefix(i.raw.Link, "https://") {
		return i.raw.Link
	}

	parsedUrl, err := url.Parse(i.feedLink)
	if err == nil {
		link := i.raw.Link
		if !strings.HasPrefix(link, "/") {
			link = "/" + link
		}
		return parsedUrl.Scheme + "://" + parsedUrl.Host + link
	}
	return i.raw.Link
}

func (i *rssFeedItem) ImageURL() string {
	if i.raw.Image != nil && i.raw.Image.URL != "" {
		return i.raw.Image.URL
	}
	if url := findThumbnailInItemExtensions(i.raw); url != "" {
		return url
	}
	return ""
}

func (i *rssFeedItem) CreatedAt() time.Time {
	if i.raw.PublishedParsed != nil {
		return *i.raw.PublishedParsed
	}
	if i.raw.UpdatedParsed != nil {
		return *i.raw.UpdatedParsed
	}
	return time.Now()
}

func (i *rssFeedItem) Categories() []string {
	categories := make([]string, 0, len(i.raw.Categories))
	for _, category := range i.raw.Categories {
		if category != "" {
			categories = append(categories, category)
		}
	}
	return categories
}

type rssFeedItemList []*rssFeedItem

func (f rssFeedItemList) sortByNewest() rssFeedItemList {
	sort.Slice(f, func(i, j int) bool {
		return f[i].CreatedAt().After(f[j].CreatedAt())
	})
	return f
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
	description, _ = common.LimitStringLength(description, 1000)
	description = sanitizeFeedDescription(description)
	description, limited := common.LimitStringLength(description, maxLen)

	if limited {
		description += "…"
	}

	return description
}
