package lib

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
	"github.com/ledongthuc/pdf"
	"github.com/rs/zerolog"
)

var (
	ErrUnsupportedContentType = errors.New("unsupported content type")
	ErrHTMLParsingFailed      = errors.New("html parsing failed")
)

func FetchThumbnailFromURL(ctx context.Context, logger *zerolog.Logger, url string) (string, error) {
	resp, err := FetchURL(ctx, logger, url)
	if err != nil {
		return "", fmt.Errorf("fetch url: %w", err)
	}

	defer resp.Body.Close()

	thumbnailURL, err := ThumbnailURLFromHTTPResponse(ctx, logger, resp)
	if err != nil {
		return "", fmt.Errorf("thumbnail from http response: %w", err)
	}

	return thumbnailURL, nil
}

func FetchTextFromURL(ctx context.Context, logger *zerolog.Logger, url string) (string, error) {
	resp, err := FetchURL(ctx, logger, url)
	if err != nil {
		return "", fmt.Errorf("fetch url: %w", err)
	}

	defer resp.Body.Close()

	text, err := TextFromHTTPResponse(ctx, logger, resp)
	if err != nil {
		return "", fmt.Errorf("text from http response: %w", err)
	}

	return text, nil
}

// FetchURL fetches a URL and returns the http response.
// The response body should be closed by the caller.
func FetchURL(ctx context.Context, logger *zerolog.Logger, url string) (*http.Response, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", DefeedUserAgentString)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch url: %w", err)
	}

	return resp, nil
}

func TextFromHTTPResponse(ctx context.Context, logger *zerolog.Logger, resp *http.Response) (string, error) {
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	url := resp.Request.URL.String()

	if strings.Contains(contentType, "application/pdf") || strings.HasSuffix(url, ".pdf") {
		return extractTextFromPDF(resp.Body)
	}

	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml+xml") {
		return extractTextFromHTML(logger, url)
	}

	logger.Warn().
		Str("url", url).
		Str("content_type", contentType).
		Msg("Unsupported content type")

	return "", ErrUnsupportedContentType
}

func extractTextFromPDF(body io.ReadCloser) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("create pdf reader: %w", err)
	}

	plainText, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("get plain text: %w", err)
	}

	textBytes, err := io.ReadAll(plainText)
	if err != nil {
		return "", fmt.Errorf("read plain text: %w", err)
	}

	return string(textBytes), nil
}

func extractTextFromHTML(logger *zerolog.Logger, url string) (string, error) {
	var result string
	var resultErr error

	defer func() {
		if r := recover(); r != nil {
			// We seem to be getting an occasional panic here.
			// Log to investigate further.
			logger.Error().
				Str("url", url).
				Interface("panic", r).
				Msg("html parsing panic")
		}
	}()

	article, err := readability.FromURL(url, 5*time.Second)
	if err != nil {
		resultErr = fmt.Errorf("readability from url: %w", err)
		return result, resultErr
	}

	result = article.TextContent
	return result, resultErr
}

// StripURL removes the protocol, www., and trailing slash from a URL.
func StripURL(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "www.")
	url = strings.TrimSuffix(url, "/")
	return url
}

func StripURLHost(url string) (string, error) {
	parsedURL, err := neturl.Parse(url)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}

	if parsedURL.Host == "" {
		return "", fmt.Errorf("url has no host")
	}

	return strings.TrimPrefix(parsedURL.Host, "www."), nil
}

// FaviconFromHTTPResponse attempts to find the favicon URL for a given website URL.
// It tries common favicon locations and parses HTML to find favicon links.
// If no favicon is found, it returns an empty string (not an error).
func FaviconFromHTTPResponse(ctx context.Context, logger *zerolog.Logger, resp *http.Response) (string, error) {
	faviconURL := findFaviconInHTML(ctx, logger, resp)
	if faviconURL != "" {
		return faviconURL, nil
	}

	// Try common favicon locations first
	commonFaviconPaths := []string{
		"/favicon.ico",
		"/favicon.png",
		"/apple-touch-icon.png",
		"/apple-touch-icon-precomposed.png",
	}

	for _, path := range commonFaviconPaths {
		faviconURL := resp.Request.URL.Scheme + "://" + resp.Request.URL.Host + path
		if checkFaviconExists(ctx, faviconURL) {
			return faviconURL, nil
		}
	}

	return "", nil
}

func checkFaviconExists(ctx context.Context, faviconURL string) bool {
	req, err := http.NewRequestWithContext(ctx, "HEAD", faviconURL, nil)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func findFaviconInHTML(ctx context.Context, logger *zerolog.Logger, resp *http.Response) string {
	websiteURL := resp.Request.URL.String()

	if resp.StatusCode != http.StatusOK {
		logger.Warn().Str("url", websiteURL).Int("status", resp.StatusCode).Msg("Non 200 status code for favicon request")
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		logger.Warn().Str("url", websiteURL).Msg("failed to parse HTML for favicon")
		return ""
	}

	// Look for favicon links in the head section
	faviconSelectors := []string{
		"link[rel='icon']",
		"link[rel='shortcut icon']",
		"link[rel='apple-touch-icon']",
		"link[rel='apple-touch-icon-precomposed']",
	}

	parsedURL, err := neturl.Parse(websiteURL)
	if err != nil {
		return ""
	}

	var foundFavicon string
	for _, selector := range faviconSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if foundFavicon != "" {
				return // Already found a favicon
			}
			if href, exists := s.Attr("href"); exists && href != "" {
				// Resolve relative URLs
				if !strings.HasPrefix(href, "http") {
					if strings.HasPrefix(href, "/") {
						href = parsedURL.Scheme + "://" + parsedURL.Host + href
					} else {
						href = parsedURL.Scheme + "://" + parsedURL.Host + "/" + href
					}
				}
				if checkFaviconExists(ctx, href) {
					foundFavicon = href
				}
			}
		})
		if foundFavicon != "" {
			break
		}
	}

	return foundFavicon
}

func ThumbnailURLFromHTTPResponse(ctx context.Context, logger *zerolog.Logger, resp *http.Response) (string, error) {
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	thumbnailURL := findThumbnailInHTML(doc, resp.Request.URL)
	if thumbnailURL != "" {
		return thumbnailURL, nil
	}

	return "", fmt.Errorf("no thumbnail found")
}

func findThumbnailInHTML(doc *goquery.Document, url *neturl.URL) string {
	thumbnailSelectors := []string{
		"meta[property='og:image']",
		"meta[name='twitter:image']",
		"meta[property='twitter:image']",
		"meta[name='og:image']",
		"link[rel='image_src']",
	}

	var foundThumbnail string
	for _, selector := range thumbnailSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if foundThumbnail != "" {
				return
			}

			var content string
			var exists bool

			if content, exists = s.Attr("content"); !exists {
				if content, exists = s.Attr("href"); !exists {
					return
				}
			}

			if content != "" {
				resolvedURL := resolveThumbnailURL(content, url)
				if resolvedURL != "" {
					foundThumbnail = resolvedURL
				}
			}
		})
		if foundThumbnail != "" {
			break
		}
	}

	return foundThumbnail
}

func resolveThumbnailURL(content string, url *neturl.URL) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	if strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") {
		return content
	}

	if strings.HasPrefix(content, "//") {
		return url.Scheme + ":" + content
	}

	if strings.HasPrefix(content, "/") {
		return url.Scheme + "://" + url.Host + content
	}

	return url.Scheme + "://" + url.Host + "/" + content
}
