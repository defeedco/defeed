package lib

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

func TestFetchFaviconURL(t *testing.T) {
	logger := zerolog.Nop()
	ctx := context.Background()

	tests := []struct {
		name              string
		url               string
		shouldHaveFavicon bool
	}{
		{
			name:              "GitHub",
			url:               "https://github.com",
			shouldHaveFavicon: true,
		},
		{
			name:              "Reddit",
			url:               "https://reddit.com",
			shouldHaveFavicon: true,
		},
		{
			name:              "Invalid URL",
			url:               "not-a-valid-url",
			shouldHaveFavicon: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FetchFaviconURL(ctx, &logger, tt.url)
			if tt.shouldHaveFavicon && result == "" {
				t.Errorf("FetchFaviconURL() returned empty string for %s, expected a favicon URL", tt.url)
			}
			if !tt.shouldHaveFavicon && result != "" {
				t.Errorf("FetchFaviconURL() returned %s for invalid URL, expected empty string", result)
			}
		})
	}
}
