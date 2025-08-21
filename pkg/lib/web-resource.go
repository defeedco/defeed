package lib

import (
	"context"
	"fmt"
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
