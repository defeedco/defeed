package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alexferrari88/gohn/pkg/gohn"
	"github.com/alitto/pond/v2"
	"github.com/defeedco/defeed/pkg/lib"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/defeedco/defeed/pkg/sources/providers"
	sourcetypes "github.com/defeedco/defeed/pkg/sources/types"
	"github.com/rs/zerolog"
)

const TypeHackerNewsPosts = "hackernewsposts"

type SourcePosts struct {
	FeedName string `json:"feedName" validate:"required,oneof=top new best ask show job"`
	client   *gohn.Client
	logger   *zerolog.Logger
}

func NewSourcePosts() *SourcePosts {
	return &SourcePosts{}
}

func (s *SourcePosts) UID() activitytypes.TypedUID {
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
	case "ask":
		return "Ask HN stories from Hacker News"
	case "show":
		return "Show HN stories from Hacker News"
	case "job":
		return "Job stories from Hacker News"
	default:
		return fmt.Sprintf("%s stories from Hacker News", lib.Capitalize(s.FeedName))
	}
}

func (s *SourcePosts) URL() string {
	switch s.FeedName {
	case "top":
		return "https://news.ycombinator.com"
	case "new":
		return "https://news.ycombinator.com/newest"
	case "job":
		return "https://news.ycombinator.com/jobs"
	default:
		return fmt.Sprintf("https://news.ycombinator.com/%s", s.FeedName)
	}
}

func (s *SourcePosts) Icon() string {
	return "https://news.ycombinator.com/favicon.ico"
}

func (s *SourcePosts) Topics() []sourcetypes.TopicTag {
	return []sourcetypes.TopicTag{sourcetypes.TopicStartups, sourcetypes.TopicDevTools, sourcetypes.TopicOpenSource}
}

func (s *SourcePosts) Validate() error { return lib.ValidateStruct(s) }

type Post struct {
	Post                *gohn.Item               `json:"post"`
	ArticleTextBody     string                   `json:"article_text_body"`
	ArticleThumbnailURL string                   `json:"article_thumbnail_url"`
	ArticleFaviconURL   string                   `json:"article_favicon_url"`
	SourceIDs           []activitytypes.TypedUID `json:"source_ids"`
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
		SourceIDs []*lib.TypedUID `json:"source_ids"`
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.SourceIDs) == 0 {
		return fmt.Errorf("source_ids is required")
	}

	p.SourceIDs = make([]activitytypes.TypedUID, len(aux.SourceIDs))
	for i, uid := range aux.SourceIDs {
		p.SourceIDs[i] = uid
	}

	return nil
}

func (p *Post) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(TypeHackerNewsPosts, fmt.Sprintf("%d", *p.Post.ID))
}

func (p *Post) SourceUIDs() []activitytypes.TypedUID {
	return p.SourceIDs
}

func (p *Post) Title() string {
	if p.Post.Title == nil {
		return ""
	}

	return *p.Post.Title
}

func (p *Post) Body() string {
	body := strings.Builder{}

	body.WriteString(p.Title())
	body.WriteString("\n\n")

	if p.Post.Text != nil {
		body.WriteString(*p.Post.Text)
		body.WriteString("\n\n")
	}

	if p.ArticleTextBody != "" {
		body.WriteString("Referenced article: \n")
		body.WriteString(p.ArticleTextBody)
	}

	return body.String()
}

func (p *Post) URL() string {
	// Note: Don't use the Post.URL, since that leads to the externally referenced page.
	return fmt.Sprintf("https://news.ycombinator.com/item?id=%d", *p.Post.ID)
}

func (p *Post) ImageURL() string {
	if p.ArticleThumbnailURL != "" {
		return p.ArticleThumbnailURL
	}

	return p.ArticleFaviconURL
}

func (p *Post) UpvotesCount() int {
	return *p.Post.Score
}

func (p *Post) DownvotesCount() int {
	return -1
}

func (p *Post) CommentsCount() int {
	return *p.Post.Descendants
}

func (p *Post) AmplificationCount() int {
	return -1
}

func (p *Post) SocialScore() float64 {
	upvotes := float64(p.UpvotesCount())
	comments := float64(p.CommentsCount())

	scoreWeight := 0.6
	commentsWeight := 0.4

	// Most popular post on HackerNews has 6k upvotes: https://hn.algolia.com/?dateRange=all&page=0&prefix=false&query=&sort=byPopularity&type=all.
	// Assume its unlikely for a post to have more comments than likes.
	maxUpvotes := 6000.0

	return (providers.NormSocialScore(upvotes, maxUpvotes) * scoreWeight) +
		(providers.NormSocialScore(comments, maxUpvotes) * commentsWeight)
}

func (p *Post) CreatedAt() time.Time {
	if p.Post.Time == nil {
		return time.Time{}
	}

	return time.Unix(int64(*p.Post.Time), 0)
}

func (s *SourcePosts) Initialize(logger *zerolog.Logger, config *sourcetypes.ProviderConfig) error {
	var err error
	s.client, err = gohn.NewClient(nil)
	if err != nil {
		return fmt.Errorf("init client: %v", err)
	}

	s.logger = logger

	return nil
}

func (s *SourcePosts) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	// Future note: we can detect post changes using https://github.com/HackerNews/API?tab=readme-ov-file#changed-items-and-profiles
	s.fetchHackerNewsPosts(ctx, since, feed, errs)
}

func (s *SourcePosts) fetchHackerNewsPosts(ctx context.Context, _ activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	storyIDs, err := s.fetchStoryIDs(ctx)

	if err != nil {
		errs <- fmt.Errorf("fetch story IDs: %v", err)
		return
	}

	if len(storyIDs) == 0 {
		errs <- fmt.Errorf("no stories found")
		return
	}

	// Note: We can't simply optimise the fetching by only retrieving since the incremental story ID,
	// because the stories are chronologically ordered only on the "new" feed timeline.
	// The order on "best" or "top" is not chronological and can change over time.
	// So for now just fetch all stories, the scheduler will skip the already processed ones.

	pool := pond.NewPool(20)

	for _, id := range storyIDs {
		if id == nil {
			continue
		}

		pool.Submit(func() {
			storyLogger := s.logger.With().
				Int("story_id", *id).
				Int("stories_count", len(storyIDs)).
				Logger()

			storyLogger.Debug().Msg("Fetching hacker news story")
			story, err := s.client.Items.Get(ctx, *id)
			if err != nil {
				storyLogger.Error().Err(err).Msg("Failed to fetch hacker news story")
				return
			}

			if story == nil {
				storyLogger.Debug().Msg("Fetched story is nil")
				return
			}

			post := &Post{
				Post:                story,
				ArticleTextBody:     "",
				ArticleThumbnailURL: "",
				ArticleFaviconURL:   "",
				SourceIDs:           []activitytypes.TypedUID{s.UID()},
			}

			if story.URL != nil {
				resp, err := lib.FetchURL(ctx, s.logger, *story.URL)
				if err != nil {
					storyLogger.Error().Err(err).Msg("Failed to fetch external article")
					return
				}

				defer resp.Body.Close()

				faviconURL, err := lib.FaviconFromHTTPResponse(ctx, s.logger, resp)
				if err == nil {
					post.ArticleFaviconURL = faviconURL
				} else {
					storyLogger.Error().Err(err).Msg("Failed to get article favicon")
				}

				thumbnailURL, err := lib.ThumbnailURLFromHTTPResponse(ctx, s.logger, resp)
				if err == nil {
					post.ArticleThumbnailURL = thumbnailURL
				} else {
					storyLogger.Error().Err(err).Msg("Failed to get article thumbnail")
				}

				content, err := lib.TextFromHTTPResponse(ctx, s.logger, resp)
				if err == nil {
					post.ArticleTextBody = content
				} else {
					storyLogger.Error().Err(err).Msg("Failed to get article text")
				}
			}

			feed <- post
		})
	}

	pool.StopAndWait()
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
	case "ask":
		storyIDs, err = s.client.Stories.GetAskIDs(ctx)
	case "show":
		storyIDs, err = s.client.Stories.GetShowIDs(ctx)
	case "job":
		storyIDs, err = s.client.Stories.GetJobIDs(ctx)
	// Note: launches (https://news.ycombinator.com/launches) is not supported by the HackerNews API,
	// so we use a RSS feed instead (https://hnrss.org/launches).
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
