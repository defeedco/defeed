package rss

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

func TestSourceFeed_Initialize(t *testing.T) {
	logger := zerolog.Nop()
	ctx := context.Background()

	tests := []struct {
		name              string
		feedURL           string
		shouldHaveFavicon bool
	}{
		{
			name:              "GitHub Blog",
			feedURL:           "https://github.blog/feed.xml",
			shouldHaveFavicon: true,
		},
		{
			name:              "URL without favicon",
			feedURL:           "https://example.com/feed.xml",
			shouldHaveFavicon: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &SourceFeed{
				FeedURL: tt.feedURL,
			}

			err := source.Initialize(ctx, &logger)
			if err != nil {
				t.Errorf("Initialize() error = %v", err)
				return
			}

			iconURL := source.Icon()

			// Verify that Icon() returns the same value as IconURL
			if iconURL != source.IconURL {
				t.Errorf("Icon() = %v, IconURL = %v, should be the same", iconURL, source.IconURL)
			}

			if tt.shouldHaveFavicon && iconURL == "" {
				t.Errorf("Icon() returned empty string for %s, expected a favicon URL", tt.feedURL)
			}
			if !tt.shouldHaveFavicon && iconURL != "" {
				t.Errorf("Icon() returned %s for invalid URL, expected empty string", iconURL)
			}
		})
	}
}
