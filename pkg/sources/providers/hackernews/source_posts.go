package hackernews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alexferrari88/gohn/pkg/gohn"
	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/rs/zerolog"
)

const TypeHackerNewsPosts = "hackernewsposts"

type SourcePosts struct {
	FeedName     string        `json:"feedName" validate:"required,oneof=top new best"`
	PollInterval time.Duration `json:"pollInterval"`
	client       *gohn.Client
	logger       *zerolog.Logger
}

func NewSourcePosts() *SourcePosts {
	return &SourcePosts{
		PollInterval: 45 * time.Minute,
	}
}

func (s *SourcePosts) UID() types.TypedUID {
	return lib.NewTypedUID(TypeHackerNewsPosts, s.FeedName)
}

func (s *SourcePosts) Name() string {
	return fmt.Sprintf("%s on Hacker News", lib.Capitalize(s.FeedName))
}

func (s *SourcePosts) Description() string {
	switch s.FeedName {
	case "top":
		return "Top trending stories from Hacker News"
	case "new":
		return "Latest new stories from Hacker News"
	case "best":
		return "Best stories from Hacker News"
	default:
		return fmt.Sprintf("%s stories from Hacker News", lib.Capitalize(s.FeedName))
	}
}

func (s *SourcePosts) URL() string {
	return fmt.Sprintf("https://news.ycombinator.com/%s", s.FeedName)
}

func (s *SourcePosts) Icon() string {
	return "https://news.ycombinator.com/favicon.ico"
}

func (s *SourcePosts) Validate() error { return lib.ValidateStruct(s) }

type Post struct {
	Post            *gohn.Item     `json:"post"`
	ArticleTextBody string         `json:"article_text_body"`
	SourceID        types.TypedUID `json:"source_id"`
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
		SourceID *lib.TypedUID `json:"source_id"`
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.SourceID == nil {
		return fmt.Errorf("source_id is required")
	}

	p.SourceID = aux.SourceID
	return nil
}

func (p *Post) UID() types.TypedUID {
	return lib.NewTypedUID(TypeHackerNewsPosts, fmt.Sprintf("%d", *p.Post.ID))
}

func (p *Post) SourceUID() types.TypedUID {
	return p.SourceID
}

func (p *Post) Title() string {
	if p.Post.Title == nil {
		return ""
	}

	return *p.Post.Title
}

func (p *Post) Body() string {
	if p.ArticleTextBody != "" {
		return p.ArticleTextBody
	}

	// Note: this is usually empty.
	if p.Post.Text != nil {
		return *p.Post.Text
	}

	return p.Title()
}

func (p *Post) URL() string {
	// Note: Don't use the Post.URL, since that leads to the externally referenced page.
	return fmt.Sprintf("https://news.ycombinator.com/item?id=%d", *p.Post.ID)
}

func (p *Post) ImageURL() string {
	return ""
}

func (p *Post) CreatedAt() time.Time {
	if p.Post.Time == nil {
		return time.Time{}
	}

	return time.Unix(int64(*p.Post.Time), 0)
}

func (s *SourcePosts) Initialize(logger *zerolog.Logger) error {
	var err error
	s.client, err = gohn.NewClient(nil)
	if err != nil {
		return fmt.Errorf("init client: %v", err)
	}

	if s.PollInterval == 0 {
		s.PollInterval = 45 * time.Minute
	}

	s.logger = logger

	return nil
}

func (s *SourcePosts) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	s.fetchHackerNewsPosts(ctx, since, feed, errs)
}

func (s *SourcePosts) fetchHackerNewsPosts(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	storyIDs, err := s.fetchStoryIDs(ctx)

	if err != nil {
		errs <- fmt.Errorf("fetch story IDs: %v", err)
		return
	}

	if len(storyIDs) == 0 {
		errs <- fmt.Errorf("no stories found")
		return
	}

	// Note: We are assuming post IDs are returned in descending order (newest first),
	// and that post IDs are incremented based on the order of the creation time.
	// This is not explicitly stated anywhere, but it seems to be the case based on the observations.
	var sincePostID int
	if since == nil {
		// Number of posts to look back for the initial fetch
		n := 100
		if len(storyIDs) >= n {
			sincePostID = *storyIDs[len(storyIDs)-n]
		} else {
			// Fallback - shouldn't happen.
			sincePostID = *storyIDs[len(storyIDs)-1]
		}
	} else {
		sincePostID = *since.(*Post).Post.ID
	}

	for _, id := range storyIDs {
		if id == nil {
			continue
		}

		storyLogger := s.logger.With().Int("story_id", *id).Logger()

		if sincePostID != 0 && *id <= sincePostID {
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
			content, err := lib.FetchTextFromURL(ctx, s.logger, *story.URL)
			if err != nil && !errors.Is(err, lib.ErrUnsupportedContentType) {
				storyLogger.Error().Err(err).Msg("Failed to fetch external article")
				continue
			}
			textContent = content
		}

		post := &Post{
			Post:            story,
			ArticleTextBody: textContent,
			SourceID:        s.UID(),
		}

		feed <- post
	}
}

func (s *SourcePosts) fetchStoryIDs(ctx context.Context) ([]*int, error) {
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

	return storyIDs, err
}

func (s *SourcePosts) MarshalJSON() ([]byte, error) {
	type Alias SourcePosts
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  TypeHackerNewsPosts,
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
