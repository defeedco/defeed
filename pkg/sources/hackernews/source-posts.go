package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/alexferrari88/gohn/pkg/gohn"
	"github.com/glanceapp/glance/pkg/lib"
	"github.com/rs/zerolog"
)

const TypeHackerNewsPosts = "hackernews:posts"

type SourcePosts struct {
	FeedName     string        `json:"feedName" validate:"required,oneof=top new best"`
	PollInterval time.Duration `json:"pollInterval"`
	client       *gohn.Client
	logger       *zerolog.Logger
}

func NewSourcePosts() *SourcePosts {
	return &SourcePosts{
		PollInterval: 5 * time.Minute,
	}
}

func (s *SourcePosts) UID() string {
	return fmt.Sprintf("%s:%s", s.Type(), s.FeedName)
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

func (s *SourcePosts) Validate() []error { return lib.ValidateStruct(s) }

type Post struct {
	Post            *gohn.Item `json:"post"`
	ArticleTextBody string     `json:"article_text_body"`
	SourceID        string     `json:"source_id"`
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
	return fmt.Sprintf("%s:%d", p.SourceID, *p.Post.ID)
}

func (p *Post) SourceUID() string {
	return p.SourceID
}

func (p *Post) Title() string {
	return *p.Post.Title
}

func (p *Post) Body() string {
	if p.ArticleTextBody != "" {
		return p.ArticleTextBody
	}

	// Note: there is also Post.Text, but its usually empty.
	return *p.Post.Title
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

func (s *SourcePosts) Initialize(logger *zerolog.Logger) error {
	var err error
	s.client, err = gohn.NewClient(nil)
	if err != nil {
		return fmt.Errorf("init client: %v", err)
	}

	if s.PollInterval == 0 {
		s.PollInterval = time.Hour
	}

	s.logger = logger

	return nil
}

func (s *SourcePosts) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	ticker := time.NewTicker(s.PollInterval)
	defer ticker.Stop()

	s.fetchAndSendNewPosts(ctx, since, feed, errs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchAndSendNewPosts(ctx, since, feed, errs)
		}
	}
}

func (s *SourcePosts) fetchAndSendNewPosts(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	posts, err := s.fetchHackerNewsPosts(ctx, since)
	if err != nil {
		errs <- fmt.Errorf("fetch posts: %v", err)
		return
	}

	for _, post := range posts {
		feed <- post
	}
}

func (s *SourcePosts) fetchHackerNewsPosts(ctx context.Context, since types.Activity) ([]*Post, error) {
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

	var sincePost *Post
	if since != nil {
		sincePost = since.(*Post)
	}

	posts := make([]*Post, 0, len(storyIDs))
	for _, id := range storyIDs {
		if id == nil {
			continue
		}

		storyLogger := s.logger.With().Int("story_id", *id).Logger()

		// Note: We are assuming post IDs are returned in descending order (newest first),
		// and that post IDs are incremented based on the order of the creation time.
		// This is not explicitly stated anywhere, but it seems to be the case based on the observations.
		if sincePost != nil && *id <= *sincePost.Post.ID {
			storyLogger.Debug().Msg("Reached last seen story")
			break
		}

		storyLogger.Debug().Msg("Fetching hacker news story")
		story, err := s.client.Items.Get(ctx, *id)
		if err != nil {
			storyLogger.Error().Err(err).Msg("Failed to fetch hacker news story")
			continue
		}

		if story == nil {
			storyLogger.Debug().Msg("Fetched story is nil")
			continue
		}

		textContent := ""
		if story.Text != nil {
			textContent = *story.Text
		} else if story.URL != nil {
			content, err := lib.FetchTextFromURL(ctx, *story.URL)
			if err != nil {
				storyLogger.Error().Err(err).Msg("Failed to fetch readable article")
				continue
			}

			textContent = content
		}

		if textContent == "" {
			storyLogger.Debug().Msg("No text content found")
		}

		posts = append(posts, &Post{
			Post:            story,
			ArticleTextBody: textContent,
			SourceID:        s.UID(),
		})
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
