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

func FetchTextFromURL(ctx context.Context, logger *zerolog.Logger, url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// TODO(config): Make this configurable
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DefeedBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")

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

// FetchFaviconURL attempts to find the favicon URL for a given website URL.
// It tries common favicon locations and parses HTML to find favicon links.
func FetchFaviconURL(ctx context.Context, logger *zerolog.Logger, websiteURL string) string {
	parsedURL, err := neturl.Parse(websiteURL)
	if err != nil {
		logger.Warn().Str("url", websiteURL).Msg("failed to parse URL for favicon")
		return ""
	}

	// Try common favicon locations first
	commonFaviconPaths := []string{
		"/favicon.ico",
		"/favicon.png",
		"/apple-touch-icon.png",
		"/apple-touch-icon-precomposed.png",
	}

	for _, path := range commonFaviconPaths {
		faviconURL := parsedURL.Scheme + "://" + parsedURL.Host + path
		if checkFaviconExists(ctx, faviconURL) {
			return faviconURL
		}
	}

	// If common locations don't work, try to parse HTML for favicon links
	return findFaviconInHTML(ctx, logger, websiteURL)
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

func findFaviconInHTML(ctx context.Context, logger *zerolog.Logger, websiteURL string) string {
	req, err := http.NewRequestWithContext(ctx, "GET", websiteURL, nil)
	if err != nil {
		logger.Warn().Str("url", websiteURL).Msg("failed to create request for favicon")
		return ""
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DefeedBot/1.0)")

	// Skip certificate verification to avoid occasional "failed to verify certificate" errors
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Warn().Str("url", websiteURL).Msg("failed to fetch HTML for favicon")
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn().Str("url", websiteURL).Int("status", resp.StatusCode).Msg("Non 200 status code for favicon request")
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
