package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/alexferrari88/gohn/pkg/gohn"
	"github.com/go-shiori/go-readability"
)

const TypeHackerNewsPosts = "hackernews-posts"

type SourcePosts struct {
	FeedName string `json:"feed_name"`
	client   *gohn.Client
}

func NewSourcePosts() *SourcePosts {
	return &SourcePosts{}
}

func (s *SourcePosts) UID() string {
	return fmt.Sprintf("hackernews/%s", s.FeedName)
}

func (s *SourcePosts) Name() string {
	return fmt.Sprintf("HackerNews (%s)", s.FeedName)
}

func (s *SourcePosts) URL() string {
	return fmt.Sprintf("https://news.ycombinator.com/%s", s.FeedName)
}

func (s *SourcePosts) Type() string {
	return TypeHackerNewsPosts
}

type Post struct {
	Post     *gohn.Item `json:"post"`
	SourceID string     `json:"source_id"`
}

func NewPost() *Post {
	return &Post{}
}

func (p *Post) SourceType() string {
	return TypeHackerNewsPosts
}

func (p *Post) MarshalJSON() ([]byte, error) {
	type Alias Post
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	})
}

func (p *Post) UnmarshalJSON(data []byte) error {
	type Alias Post
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	return json.Unmarshal(data, &aux)
}

func (p *Post) UID() string {
	return fmt.Sprintf("%d", *p.Post.ID)
}

func (p *Post) SourceUID() string {
	return p.SourceID
}

func (p *Post) Title() string {
	return *p.Post.Title
}

func (p *Post) Body() string {
	body := *p.Post.Title
	if p.Post.URL != nil {
		article, err := readability.FromURL(*p.Post.URL, 5*time.Second)
		if err == nil {
			body += fmt.Sprintf("\n\nReferenced article: \n%s", article.TextContent)
		} else {
			slog.Error("Failed to fetch hacker news article", "error", err, "url", *p.Post.URL)
		}
	}
	return body
}

func (p *Post) URL() string {
	if p.Post.URL != nil {
		return *p.Post.URL
	}
	return fmt.Sprintf("https://news.ycombinator.com/item?id=%d", *p.Post.ID)
}

func (p *Post) ImageURL() string {
	return ""
}

func (p *Post) CreatedAt() time.Time {
	return time.Unix(int64(*p.Post.Time), 0)
}

func (s *SourcePosts) Initialize() error {
	if s.FeedName != "top" && s.FeedName != "new" && s.FeedName != "best" {
		return fmt.Errorf("feed name must be one of: 'top', 'new', 'best'")
	}

	var err error
	s.client, err = gohn.NewClient(nil)
	if err != nil {
		return fmt.Errorf("init client: %v", err)
	}

	return nil
}

func (s *SourcePosts) Stream(ctx context.Context, feed chan<- types.Activity, errs chan<- error) {
	posts, err := s.fetchHackerNewsPosts(ctx)

	if err != nil {
		errs <- fmt.Errorf("fetch posts: %v", err)
		return
	}

	for _, post := range posts {
		feed <- post
	}

}

func (s *SourcePosts) fetchHackerNewsPosts(ctx context.Context) ([]*Post, error) {
	var storyIDs []*int
	var err error

	switch s.FeedName {
	case "top":
		storyIDs, err = s.client.Stories.GetTopIDs(ctx)
	case "new":
		storyIDs, err = s.client.Stories.GetNewIDs(ctx)
	case "best":
		storyIDs, err = s.client.Stories.GetBestIDs(ctx)
	default:
		return nil, fmt.Errorf("invalid feed name: %s", s.FeedName)
	}

	if err != nil {
		return nil, fmt.Errorf("fetch story IDs: %v", err)
	}

	if len(storyIDs) == 0 {
		return nil, fmt.Errorf("no stories found")
	}

	posts := make([]*Post, 0, len(storyIDs))
	for _, id := range storyIDs {
		if id == nil {
			continue
		}

		story, err := s.client.Items.Get(ctx, *id)
		if err != nil {
			slog.Error("Failed to fetch hacker news story", "error", err, "id", *id)
			continue
		}

		if story == nil {
			continue
		}

		posts = append(posts, &Post{Post: story, SourceID: s.UID()})
	}

	if len(posts) == 0 {
		return nil, fmt.Errorf("no valid stories found")
	}

	return posts, nil
}

func (s *SourcePosts) MarshalJSON() ([]byte, error) {
	type Alias SourcePosts
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  s.Type(),
	})
}

func (s *SourcePosts) UnmarshalJSON(data []byte) error {
	type Alias SourcePosts
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
