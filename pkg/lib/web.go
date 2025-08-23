package lib

import (
	"context"
	"fmt"
	neturl "net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

func FetchTextFromURL(_ context.Context, url string) (string, error) {
	// TODO: add support for fetching non-HTML content (e.g. PDFs)
	article, err := readability.FromURL(url, 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("readability from url: %w", err)
	}
	return article.TextContent, nil
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
