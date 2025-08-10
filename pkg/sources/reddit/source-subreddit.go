package reddit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/go-shiori/go-readability"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

const TypeRedditSubreddit = "reddit-subreddit"

type SourceSubreddit struct {
	Subreddit          string `json:"subreddit"`
	SortBy             string `json:"sortBy"`
	TopPeriod          string `json:"topPeriod"`
	Search             string `json:"search"`
	RequestURLTemplate string `json:"requestUrlTemplate"`
	client             *reddit.Client
	AppAuth            struct {
		Name   string `json:"name"`
		ID     string `json:"ID"`
		Secret string `json:"secret"`
	} `json:"auth"`
}

func NewSourceSubreddit() *SourceSubreddit {
	return &SourceSubreddit{}
}

func (s *SourceSubreddit) UID() string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", TypeRedditSubreddit, s.Subreddit, s.SortBy, s.TopPeriod, s.Search)
}

func (s *SourceSubreddit) Name() string {
	return fmt.Sprintf("Reddit (%s, %s, %s)", s.Subreddit, s.SortBy, s.TopPeriod)
}

func (s *SourceSubreddit) URL() string {
	return fmt.Sprintf("https://reddit.com/r/%s/%s", s.Subreddit, s.SortBy)
}

func (s *SourceSubreddit) Type() string {
	return TypeRedditSubreddit
}

type Post struct {
	Post      *reddit.Post `json:"post"`
	SourceID  string       `json:"source_id"`
	SourceTyp string       `json:"source_type"`
}

func NewPost() *Post {
	return &Post{}
}

func (p *Post) SourceType() string {
	return p.SourceTyp
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
	return p.Post.ID
}

func (p *Post) SourceUID() string {
	return p.SourceID
}

func (p *Post) Title() string {
	return html.UnescapeString(p.Post.Title)
}

func (p *Post) Body() string {
	body := p.Post.Body
	if p.Post.URL != "" && !p.Post.IsSelfPost {
		article, err := readability.FromURL(p.Post.URL, 5*time.Second)
		if err == nil {
			body += fmt.Sprintf("\n\nReferenced article: \n%s", article.TextContent)
		} else {
			slog.Error("Failed to fetch reddit article", "error", err, "url", p.Post.URL)
		}
	}
	return body
}

func (p *Post) URL() string {
	// TODO(pulse): Test format
	return "https://www.reddit.com" + p.Post.Permalink
}

func (p *Post) ImageURL() string {
	// TODO(pulse): Fetch thumbnail URL
	// The go-reddit library doesn't provide direct access to thumbnail URLs
	// We'll need to fetch this information separately if needed
	return ""
}

func (p *Post) CreatedAt() time.Time {
	return p.Post.Created.Time
}

func (s *SourceSubreddit) Initialize() error {
	if s.Subreddit == "" {
		return errors.New("subreddit is required")
	}

	sort := s.SortBy
	if sort != "hot" && sort != "new" && sort != "top" && sort != "rising" {
		return errors.New("sort by must be one of: 'hot', 'new', 'top', 'rising'")
	}

	p := s.TopPeriod
	if p != "hour" && p != "day" && p != "week" && p != "month" && p != "year" && p != "all" {
		return errors.New("top period must be one of: 'hour', 'day', 'week', 'month', 'year', 'all'")
	}

	if s.RequestURLTemplate != "" {
		if !strings.Contains(s.RequestURLTemplate, "{REQUEST-URL}") {
			return errors.New("no `{REQUEST-URL}` placeholder specified")
		}
	}

	var client *reddit.Client
	var err error

	if s.AppAuth.ID != "" && s.AppAuth.Secret != "" {
		client, err = reddit.NewClient(reddit.Credentials{
			ID:     s.AppAuth.ID,
			Secret: s.AppAuth.Secret,
		})
	} else {
		client, err = reddit.NewReadonlyClient()
	}

	if err != nil {
		return fmt.Errorf("create reddit client: %v", err)
	}

	s.client = client

	return nil
}

func (s *SourceSubreddit) Stream(ctx context.Context, feed chan<- types.Activity, errs chan<- error) {
	posts, err := s.fetchSubredditPosts(ctx)

	if err != nil {
		errs <- fmt.Errorf("fetch posts: %v", err)
		return
	}

	for _, post := range posts {
		feed <- post
	}
}

func (s *SourceSubreddit) fetchSubredditPosts(ctx context.Context) ([]*Post, error) {
	var posts []*reddit.Post
	var err error

	limit := 10

	opts := &reddit.ListOptions{
		Limit: limit,
	}

	if s.Search != "" {
		searchOpts := &reddit.ListPostSearchOptions{
			ListPostOptions: reddit.ListPostOptions{
				ListOptions: reddit.ListOptions{
					Limit: limit,
				},
			},
			Sort: s.SortBy,
		}
		posts, _, err = s.client.Subreddit.SearchPosts(ctx, s.Subreddit, s.Search, searchOpts)
	} else {
		switch s.SortBy {
		case "hot":
			posts, _, err = s.client.Subreddit.HotPosts(ctx, s.Subreddit, opts)
		case "new":
			posts, _, err = s.client.Subreddit.NewPosts(ctx, s.Subreddit, opts)
		case "top":
			topOpts := &reddit.ListPostOptions{
				ListOptions: reddit.ListOptions{
					Limit: limit,
				},
				Time: s.TopPeriod,
			}
			posts, _, err = s.client.Subreddit.TopPosts(ctx, s.Subreddit, topOpts)
		case "rising":
			posts, _, err = s.client.Subreddit.RisingPosts(ctx, s.Subreddit, opts)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("fetching posts: %v", err)
	}

	if len(posts) == 0 {
		return nil, fmt.Errorf("no posts found")
	}

	redditPosts := make([]*Post, 0, len(posts))
	for _, post := range posts {
		if post.Stickied {
			continue
		}

		redditPosts = append(redditPosts, &Post{Post: post, SourceTyp: s.Type(), SourceID: s.UID()})
	}

	return redditPosts, nil
}

func (s *SourceSubreddit) MarshalJSON() ([]byte, error) {
	type Alias SourceSubreddit
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  s.Type(),
	})
}

func (s *SourceSubreddit) UnmarshalJSON(data []byte) error {
	type Alias SourceSubreddit
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
