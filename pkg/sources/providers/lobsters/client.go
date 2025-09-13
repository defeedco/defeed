package lobsters

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/defeedco/defeed/pkg/lib"
)

// There is no official REST API, but each page can be fetched as JSON.
// See: https://lobste.rs/s/r9oskz/is_there_api_documentation_for_lobsters
type LobstersClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewLobstersClient(baseURL string) *LobstersClient {
	if baseURL == "" {
		baseURL = "https://lobste.rs"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &LobstersClient{
		httpClient: lib.DefaultHTTPClient,
		baseURL:    baseURL,
	}
}

type Story struct {
	ID           string    `json:"short_id"`
	CreatedAt    time.Time `json:"created_at"`
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	Score        int       `json:"score"`
	CommentCount int       `json:"comment_count"`
	CommentsURL  string    `json:"comments_url"`
	Tags         []string  `json:"tags"`
}

func (c *LobstersClient) GetStoriesByFeed(ctx context.Context, feed string) ([]*Story, error) {
	url := fmt.Sprintf("%s/%s.json", c.baseURL, feed)
	return c.fetchStories(ctx, url)
}

func (c *LobstersClient) GetStoriesByTag(ctx context.Context, tag string) ([]*Story, error) {
	url := fmt.Sprintf("%s/t/%s.json", c.baseURL, tag)
	return c.fetchStories(ctx, url)
}

func (c *LobstersClient) fetchStories(ctx context.Context, url string) ([]*Story, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %v", err)
	}

	var stories []*Story
	stories, err = lib.DecodeJSONFromRequest[[]*Story](c.httpClient, req)
	if err != nil {
		return nil, fmt.Errorf("fetching stories: %v", err)
	}

	return stories, nil
}
