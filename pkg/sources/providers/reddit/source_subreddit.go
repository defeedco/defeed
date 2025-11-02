package reddit

import (
	"context"
	"encoding/json"
	"errors"
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

const TypeRedditSubreddit = "redditsubreddit"

type SourceSubreddit struct {
	Subreddit        string `json:"subreddit" validate:"required"`
	SubredditSummary string `json:"subredditSummary"`
	SortBy           string `json:"sortBy" validate:"required,oneof=hot new top rising"`
	TopPeriod        string `json:"topPeriod" validate:"required,oneof=hour day week month year all"`
	Search           string `json:"search"`
	client           *reddit.Client
	logger           *zerolog.Logger
}

func NewSourceSubreddit() *SourceSubreddit {
	return &SourceSubreddit{}
}

func (s *SourceSubreddit) UID() activitytypes.TypedUID {
	if s.Search != "" {
		return lib.NewTypedUID(TypeRedditSubreddit, s.Subreddit, s.SortBy, s.TopPeriod, s.Search)
	}
	return lib.NewTypedUID(TypeRedditSubreddit, s.Subreddit, s.SortBy, s.TopPeriod)
}

func (s *SourceSubreddit) Name() string {
	return fmt.Sprintf("%s on r/%s", lib.Capitalize(s.SortBy), s.Subreddit)
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
		tags = append(tags, sourcetypes.TopicLargeLanguageModels, sourcetypes.TopicArtificialIntelligence)
	case "machinelearning", "deeplearning":
		tags = append(tags, sourcetypes.TopicArtificialIntelligence)
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
	case "pennystocks", "wsbafterhours", "10xpennystocks":
		tags = append(tags, sourcetypes.TopicFinance)
	}
	return tags
}

type Post struct {
	Post            *reddit.Post             `json:"post"`
	ThumbnailURL    string                   `json:"thumbnail_url"`
	ExternalContent string                   `json:"external_content"`
	SourceIDs       []activitytypes.TypedUID `json:"source_ids"`
	SourceTyp       string                   `json:"source_type"`
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
	return lib.NewTypedUID(p.SourceTyp, p.Post.ID)
}

func (p *Post) SourceUIDs() []activitytypes.TypedUID {
	return p.SourceIDs
}

func (p *Post) Title() string {
	return html.UnescapeString(p.Post.Title)
}

func (p *Post) Body() string {
	sb := strings.Builder{}
	sb.WriteString(p.Post.Body)
	if p.ExternalContent != "" {
		sb.WriteString("\n\nExternal link content:\n")
		sb.WriteString(p.ExternalContent)
	}
	return sb.String()
}

func (p *Post) URL() string {
	// TODO(pulse): Test format
	return "https://www.reddit.com" + p.Post.Permalink
}

func (p *Post) ImageURL() string {
	return p.ThumbnailURL
}

func (p *Post) CreatedAt() time.Time {
	return p.Post.Created.Time
}

func (p *Post) UpvotesCount() int {
	return p.Post.Score
}

func (p *Post) DownvotesCount() int {
	return -1
}

func (p *Post) CommentsCount() int {
	return p.Post.NumberOfComments
}

func (p *Post) AmplificationCount() int {
	return -1
}

func (p *Post) SocialScore() float64 {
	score := float64(p.UpvotesCount())
	comments := float64(p.CommentsCount())

	scoreWeight := 0.6
	commentsWeight := 0.4

	maxScore := 10000.0
	maxComments := 1000.0

	return (providers.NormSocialScore(score, maxScore) * scoreWeight) +
		(providers.NormSocialScore(comments, maxComments) * commentsWeight)
}

func (s *SourceSubreddit) Initialize(logger *zerolog.Logger, config *sourcetypes.ProviderConfig) error {
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

func (s *SourceSubreddit) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	// Fetch posts from subreddit RSS feed until we get access to the Reddit API to avoid rate limit issues.
	useRSS := true
	if useRSS {
		s.fetchSubredditPostsWithRSS(ctx, since, feed, errs)
	} else {
		s.fetchSubredditPosts(ctx, since, feed, errs)
	}
}

func (s *SourceSubreddit) fetchSubredditPostsWithRSS(ctx context.Context, _ activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	parser := gofeed.NewParser()
	parser.UserAgent = lib.DefeedUserAgentString

	rssFeed, err := parser.ParseURLWithContext(fmt.Sprintf("https://www.reddit.com/r/%s.rss", s.Subreddit), ctx)
	if err != nil {
		errs <- fmt.Errorf("fetch rss feed: %w", err)
		return
	}

	if rssFeed == nil {
		errs <- fmt.Errorf("feed is nil")
		return
	}

	if len(rssFeed.Items) == 0 {
		return
	}

	for _, item := range rssFeed.Items {
		if item.PublishedParsed == nil {
			s.logger.Warn().Msgf("skipping item with no published date: %+v", item)
			continue
		}

		postID := strings.TrimPrefix(item.GUID, "t3_")

		feedItem := &Post{
			Post: &reddit.Post{
				ID:        postID,
				Title:     item.Title,
				URL:       item.Link,
				Created:   &reddit.Timestamp{Time: *item.PublishedParsed},
				Permalink: strings.TrimPrefix(item.Link, "https://www.reddit.com"),
				Score:     -1,
			},
			ExternalContent: "",
			// The Reddit share link returns the default reddit thumbnail for some reason,
			// so we use a Slack image proxy to get the actual thumbnail.
			// ThumbnailURL:    fmt.Sprintf("https://share.redd.it/preview/post/%s", postID),
			ThumbnailURL: fmt.Sprintf("https://slack-imgs.com/?c=1&o1=ro&url=https://share.redd.it/preview/post/%s", postID),
			SourceTyp:    TypeRedditSubreddit,
			SourceIDs:    []activitytypes.TypedUID{s.UID()},
		}

		feed <- feedItem
	}
}

func (s *SourceSubreddit) fetchSubredditPosts(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	event := s.logger.With().
		Str("subreddit", s.Subreddit).
		Str("sort_by", s.SortBy).
		Str("top_period", s.TopPeriod).
		Str("search", s.Search).
		Logger()

	var sinceID string
	if since != nil {
		sinceID = since.(*Post).Post.FullID
	} else {
		event.Debug().Msg("Fetching recent posts")
		// If this is the first time we're fetching posts,
		// only fetch the last few posts to avoid retrieving all historic posts.
		s.fetchRecentPosts(ctx, feed, errs)
		return
	}

outer:
	for {
		event.Debug().Msg("Fetching posts")
		redditPosts, _, err := s.fetchByCurrentTimeline(ctx, &reddit.ListOptions{
			Limit: 10,
			After: sinceID,
		})
		if err != nil {
			errs <- fmt.Errorf("fetch posts: %v", err)
			return
		}

		event.Debug().Int("count", len(redditPosts)).Msg("Fetched posts")

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
		SourceIDs:       []activitytypes.TypedUID{s.UID()},
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
