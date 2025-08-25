package lib

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

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
