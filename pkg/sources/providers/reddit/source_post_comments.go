package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/defeedco/defeed/pkg/lib"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/defeedco/defeed/pkg/sources/providers"
	sourcetypes "github.com/defeedco/defeed/pkg/sources/types"
	"github.com/mmcdole/gofeed"
	"github.com/rs/zerolog"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

const TypeRedditPostComments = "redditpostcomments"

type SourcePostComments struct {
	PostPermalink string `json:"post_permalink" validate:"required"`
	client        *reddit.Client
	logger        *zerolog.Logger
}

func NewSourcePostComments() *SourcePostComments {
	return &SourcePostComments{}
}

func (s *SourcePostComments) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(TypeRedditPostComments, s.PostPermalink)
}

func (s *SourcePostComments) Name() string {
	return fmt.Sprintf("Comments on %s", s.PostPermalink)
}

func (s *SourcePostComments) Description() string {
	return fmt.Sprintf("Comments from Reddit post %s", s.PostPermalink)
}

func (s *SourcePostComments) URL() string {
	return fmt.Sprintf("https://www.reddit.com%s", s.PostPermalink)
}

func (s *SourcePostComments) Icon() string {
	return "https://reddit.com/favicon.ico"
}

func (s *SourcePostComments) Topics() []sourcetypes.TopicTag {
	return []sourcetypes.TopicTag{}
}

type Comment struct {
	ID           string                   `json:"id"`
	Author       string                   `json:"author"`
	BodyText     string                   `json:"body_text"`
	Score        int                      `json:"score"`
	Permalink    string                   `json:"permalink"`
	Created      time.Time                `json:"created"`
	ParentID     string                   `json:"parent_id"`
	SourceTyp    string                   `json:"source_type"`
	ParentUIDVal activitytypes.TypedUID   `json:"parent_uid"`
	SourceIDs    []activitytypes.TypedUID `json:"source_ids"`
}

func NewComment() *Comment {
	return &Comment{}
}

func (c *Comment) SourceType() string {
	return c.SourceTyp
}

func (c *Comment) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(c.SourceTyp, c.ID)
}

func (c *Comment) SourceUIDs() []activitytypes.TypedUID {
	return c.SourceIDs
}

func (c *Comment) ParentUID() *activitytypes.TypedUID {
	return &c.ParentUIDVal
}

func (c *Comment) ChildUIDs() []activitytypes.TypedUID {
	return []activitytypes.TypedUID{}
}

func (c *Comment) Title() string {
	return fmt.Sprintf("Comment by u/%s", c.Author)
}

func (c *Comment) Body() string {
	return html.UnescapeString(c.BodyText)
}

func (c *Comment) URL() string {
	return fmt.Sprintf("https://www.reddit.com%s", c.Permalink)
}

func (c *Comment) ImageURL() string {
	return ""
}

func (c *Comment) CreatedAt() time.Time {
	return c.Created
}

func (c *Comment) UpvotesCount() int {
	return c.Score
}

func (c *Comment) DownvotesCount() int {
	return -1
}

func (c *Comment) CommentsCount() int {
	return -1
}

func (c *Comment) AmplificationCount() int {
	return -1
}

func (c *Comment) SocialScore() float64 {
	score := float64(c.UpvotesCount())
	maxScore := 1000.0

	return providers.NormSocialScore(score, maxScore)
}

func (c *Comment) MarshalJSON() ([]byte, error) {
	type Alias Comment
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	})
}

func (c *Comment) UnmarshalJSON(data []byte) error {
	type Alias Comment
	aux := &struct {
		*Alias
		SourceIDs []*lib.TypedUID `json:"source_ids"`
		ParentUID *lib.TypedUID   `json:"parent_uid"`
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.SourceIDs) == 0 {
		return fmt.Errorf("source_ids is required")
	}

	c.SourceIDs = make([]activitytypes.TypedUID, len(aux.SourceIDs))
	for i, uid := range aux.SourceIDs {
		c.SourceIDs[i] = uid
	}

	if aux.ParentUID != nil {
		c.ParentUIDVal = aux.ParentUID
	}

	return nil
}

func (s *SourcePostComments) Initialize(logger *zerolog.Logger, config *sourcetypes.ProviderConfig) error {
	var client *reddit.Client
	var err error

	if config.RedditClientID != "" && config.RedditClientSecret != "" {
		client, err = reddit.NewClient(reddit.Credentials{
			ID:     config.RedditClientID,
			Secret: config.RedditClientSecret,
		})
	} else {
		client, err = reddit.NewReadonlyClient()
	}

	if err != nil {
		return fmt.Errorf("create reddit client: %v", err)
	}

	s.client = client
	s.logger = logger

	return nil
}

func (s *SourcePostComments) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	s.fetchCommentsWithRSS(ctx, since, feed, errs)
}

func (s *SourcePostComments) fetchCommentsWithRSS(ctx context.Context, _ activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	parser := gofeed.NewParser()
	parser.UserAgent = lib.DefeedUserAgentString

	rssURL := fmt.Sprintf("https://www.reddit.com%s/.rss", s.PostPermalink)
	rssFeed, err := parser.ParseURLWithContext(rssURL, ctx)
	if err != nil {
		errs <- fmt.Errorf("fetch comments rss feed: %w", err)
		return
	}

	if rssFeed == nil {
		errs <- fmt.Errorf("comments feed is nil")
		return
	}

	if len(rssFeed.Items) == 0 {
		return
	}

	for _, item := range rssFeed.Items {
		if item.PublishedParsed == nil {
			continue
		}

		commentID := strings.TrimPrefix(item.GUID, "t3_")

		parentUID, err := s.getParentUIDFromContext(ctx)
		if err != nil {
			errs <- fmt.Errorf("get parent UID from context: %w", err)
			return
		}

		permalink := s.PostPermalink
		if item.Link != "" {
			permalink = strings.TrimPrefix(item.Link, "https://www.reddit.com")
		}

		var author string
		if item.Author != nil {
			author = strings.TrimPrefix(item.Author.Name, "/u/")
		}

		feedItem := &Comment{
			ID:           commentID,
			Author:       author,
			BodyText:     item.Content,
			Score:        -1,
			Permalink:    permalink,
			Created:      *item.PublishedParsed,
			SourceTyp:    TypeRedditPostComments,
			ParentUIDVal: parentUID,
			SourceIDs:    []activitytypes.TypedUID{s.UID()},
		}

		feed <- feedItem
	}
}

// TODO: This is only temporary, parent ID should be passed as an argument to Stream.
func (s *SourcePostComments) getParentUIDFromContext(ctx context.Context) (activitytypes.TypedUID, error) {
	if val := ctx.Value("parent_uid"); val != nil {
		parentUID, ok := val.(activitytypes.TypedUID)
		if !ok {
			return nil, fmt.Errorf("parent UID is not a typed UID")
		}
		return parentUID, nil
	}
	return nil, fmt.Errorf("parent UID is not set in context")
}

func (s *SourcePostComments) MarshalJSON() ([]byte, error) {
	type Alias SourcePostComments
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  TypeRedditPostComments,
	})
}

func (s *SourcePostComments) UnmarshalJSON(data []byte) error {
	type Alias SourcePostComments
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
