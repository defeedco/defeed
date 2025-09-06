package reddit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	activitytypes "github.com/glanceapp/glance/pkg/sources/activities/types"
	sourcetypes "github.com/glanceapp/glance/pkg/sources/types"
	"github.com/rs/zerolog"
	"github.com/vartanbeno/go-reddit/v2/reddit"
)

const TypeRedditSubreddit = "redditsubreddit"

type SourceSubreddit struct {
	Subreddit        string `json:"subreddit" validate:"required"`
	SubredditSummary string `json:"subredditSummary"`
	SortBy           string `json:"sortBy" validate:"required,oneof=hot new top rising"`
	TopPeriod        string `json:"topPeriod" validate:"required,oneof=hour day week month year all"`
	Search           string `json:"search"`
	client           *reddit.Client
	AppAuth          struct {
		Name   string `json:"name"`
		ID     string `json:"ID"`
		Secret string `json:"secret" validate:"required_with=ID"`
	} `json:"auth"`
	logger *zerolog.Logger
}

func NewSourceSubreddit() *SourceSubreddit {
	return &SourceSubreddit{}
}

func (s *SourceSubreddit) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(TypeRedditSubreddit, s.Subreddit, s.SortBy, s.TopPeriod, s.Search)
}

func (s *SourceSubreddit) Name() string {
	return fmt.Sprintf("%s subreddit", lib.Capitalize(s.Subreddit))
}

func (s *SourceSubreddit) Description() string {
	return fmt.Sprintf("%s %s posts from r/%s", lib.Capitalize(reformatTopPeriod(s.TopPeriod)), s.SortBy, s.Subreddit)
}

func reformatTopPeriod(value string) string {
	switch value {
	case "hour":
		return "hourly"
	case "day":
		return "daily"
	case "week":
		return "weekly"
	case "month":
		return "monthly"
	case "year":
		return "yearly"
	case "all":
		return "all time"
	}
	return value
}

func (s *SourceSubreddit) URL() string {
	return fmt.Sprintf("https://reddit.com/r/%s/%s", s.Subreddit, s.SortBy)
}

func (s *SourceSubreddit) Icon() string {
	return "https://reddit.com/favicon.ico"
}

func (s *SourceSubreddit) Topics() []sourcetypes.TopicTag {
	tags := []sourcetypes.TopicTag{}
	switch strings.ToLower(s.Subreddit) {
	case "chatgpt", "local_llama", "localllama", "local_llms", "llama", "openai":
		tags = append(tags, sourcetypes.TopicLLMs, sourcetypes.TopicAIResearch)
	case "machinelearning", "deeplearning":
		tags = append(tags, sourcetypes.TopicAIResearch)
	case "javascript", "reactjs", "webdev":
		tags = append(tags, sourcetypes.TopicDevTools, sourcetypes.TopicWebPerformance)
	case "golang", "rust", "programming":
		tags = append(tags, sourcetypes.TopicSystemsProgramming, sourcetypes.TopicOpenSource)
	case "startups", "entrepreneur":
		tags = append(tags, sourcetypes.TopicStartups, sourcetypes.TopicGrowthEngineering)
	case "kubernetes", "devops":
		tags = append(tags, sourcetypes.TopicCloudInfrastructure, sourcetypes.TopicDistributedSystems)
	case "linux":
		tags = append(tags, sourcetypes.TopicSystemsProgramming, sourcetypes.TopicOpenSource)
	}
	return tags
}

type Post struct {
	Post            *reddit.Post            `json:"post"`
	ExternalContent string                  `json:"external_content"`
	SourceID        activitytypes.TypedUID  `json:"source_id"`
	SourceTyp       string                  `json:"source_type"`
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

func (p *Post) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(p.SourceTyp, p.Post.ID)
}

func (p *Post) SourceUID() activitytypes.TypedUID {
	return p.SourceID
}

func (p *Post) Title() string {
	return html.UnescapeString(p.Post.Title)
}

func (p *Post) Body() string {
	return fmt.Sprintf("%s\n\nExternal link content:\n%s", p.Post.Body, p.ExternalContent)
}

func (p *Post) URL() string {
	// TODO(pulse): Test format
	return "https://www.reddit.com" + p.Post.Permalink
}

func (p *Post) ImageURL() string {
	return ""
}

func (p *Post) CreatedAt() time.Time {
	return p.Post.Created.Time
}

func (s *SourceSubreddit) Initialize(logger *zerolog.Logger) error {
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

	s.logger = logger

	return nil
}

func (s *SourceSubreddit) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	s.fetchSubredditPosts(ctx, since, feed, errs)
}

func (s *SourceSubreddit) fetchSubredditPosts(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	subrLogger := s.logger.With().
		Str("subreddit", s.Subreddit).
		Str("sort_by", s.SortBy).
		Str("top_period", s.TopPeriod).
		Str("search", s.Search).
		Logger()

	var sinceID string
	if since != nil {
		sinceID = since.(*Post).Post.FullID
	} else {
		subrLogger.Debug().Msg("Fetching recent posts")
		// If this is the first time we're fetching posts,
		// only fetch the last few posts to avoid retrieving all historic posts.
		s.fetchRecentPosts(ctx, feed, errs)
		return
	}

outer:
	for {
		subrLogger.Debug().Msg("Fetching posts")
		redditPosts, _, err := s.fetchByCurrentTimeline(ctx, &reddit.ListOptions{
			Limit: 10,
			After: sinceID,
		})
		if err != nil {
			errs <- fmt.Errorf("fetch posts: %v", err)
			return
		}

		subrLogger.Debug().Int("count", len(redditPosts)).Msg("Fetched posts")

		if len(redditPosts) == 0 {
			break outer
		}

		for _, post := range redditPosts {
			// Skip pineed posts
			if post.Stickied {
				continue
			}
			// Skip NSFW posts to avoid missuse or legal issues
			if post.NSFW {
				continue
			}
			builtPost, err := s.buildPost(ctx, post)
			if err != nil {
				errs <- fmt.Errorf("build post: %v", err)
				return
			}
			feed <- builtPost
		}

		sinceID = redditPosts[len(redditPosts)-1].FullID
	}
}

func (s *SourceSubreddit) fetchRecentPosts(ctx context.Context, feed chan<- activitytypes.Activity, errs chan<- error) {
	redditPosts, _, err := s.fetchByCurrentTimeline(ctx, &reddit.ListOptions{
		Limit: 10,
	})
	if err != nil {
		errs <- fmt.Errorf("fetch posts: %v", err)
		return
	}

	for _, post := range redditPosts {
		builtPost, err := s.buildPost(ctx, post)
		if err != nil {
			errs <- fmt.Errorf("build post: %v", err)
			return
		}
		feed <- builtPost
	}
}

func (s *SourceSubreddit) buildPost(ctx context.Context, post *reddit.Post) (*Post, error) {
	externalContent := ""

	// Note: self post is a post that doesn't link outside of reddit.com
	if post.URL != "" && !post.IsSelfPost {
		content, err := lib.FetchTextFromURL(ctx, s.logger, post.URL)

		// It's okay to skip unsupported content types (e.g. images)
		if err != nil && !errors.Is(err, lib.ErrUnsupportedContentType) {
			return nil, fmt.Errorf("fetch external content: %w", err)
		}

		externalContent = content
	}

	return &Post{
		Post:            post,
		ExternalContent: externalContent,
		SourceTyp:       TypeRedditSubreddit,
		SourceID:        s.UID(),
	}, nil
}

func (s *SourceSubreddit) fetchByCurrentTimeline(ctx context.Context, opts *reddit.ListOptions) ([]*reddit.Post, *reddit.Response, error) {
	if s.Search != "" {
		searchOpts := &reddit.ListPostSearchOptions{
			ListPostOptions: reddit.ListPostOptions{
				ListOptions: *opts,
			},
			Sort: s.SortBy,
		}
		return s.client.Subreddit.SearchPosts(ctx, s.Subreddit, s.Search, searchOpts)
	}

	switch s.SortBy {
	case "hot":
		return s.client.Subreddit.HotPosts(ctx, s.Subreddit, opts)
	case "new":
		return s.client.Subreddit.NewPosts(ctx, s.Subreddit, opts)
	case "top":
		topOpts := &reddit.ListPostOptions{
			ListOptions: *opts,
			Time:        s.TopPeriod,
		}
		return s.client.Subreddit.TopPosts(ctx, s.Subreddit, topOpts)
	case "rising":
		return s.client.Subreddit.RisingPosts(ctx, s.Subreddit, opts)
	}

	return nil, nil, fmt.Errorf("invalid sort by: %s", s.SortBy)
}

func (s *SourceSubreddit) MarshalJSON() ([]byte, error) {
	type Alias SourceSubreddit
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  TypeRedditSubreddit,
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
