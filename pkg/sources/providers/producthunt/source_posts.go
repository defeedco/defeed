package producthunt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/defeedco/defeed/pkg/lib"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/defeedco/defeed/pkg/sources/providers"
	sourcetypes "github.com/defeedco/defeed/pkg/sources/types"
	"github.com/rs/zerolog"
)

const TypeProductHuntPosts = "producthuntposts"

type SourcePosts struct {
	FeedName string `json:"feedName" validate:"required,oneof=new top"`
	client   *Client
	logger   *zerolog.Logger
}

func NewSourcePosts() *SourcePosts {
	return &SourcePosts{}
}

func (s *SourcePosts) UID() activitytypes.TypedUID {
	return lib.NewTypedUID(TypeProductHuntPosts, s.FeedName)
}

func (s *SourcePosts) Name() string {
	return fmt.Sprintf("%s on Product Hunt", lib.Capitalize(s.FeedName))
}

func (s *SourcePosts) Description() string {
	switch s.FeedName {
	case "top":
		return "Top trending products from Product Hunt"
	case "new":
		return "Latest new products from Product Hunt"
	default:
		return fmt.Sprintf("%s products from Product Hunt", lib.Capitalize(s.FeedName))
	}
}

func (s *SourcePosts) URL() string {
	return "https://www.producthunt.com"
}

func (s *SourcePosts) Icon() string {
	return "https://www.producthunt.com/favicon.ico"
}

func (s *SourcePosts) Topics() []sourcetypes.TopicTag {
	return []sourcetypes.TopicTag{sourcetypes.TopicStartups, sourcetypes.TopicDevTools, sourcetypes.TopicProductManagement}
}

func (s *SourcePosts) Validate() error {
	return lib.ValidateStruct(s)
}

type Post struct {
	Product   *PostNode                `json:"product"`
	SourceIDs []activitytypes.TypedUID `json:"source_ids"`
}

func NewPost() *Post {
	return &Post{}
}

func (p *Post) SourceType() string {
	return TypeProductHuntPosts
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
	return lib.NewTypedUID(TypeProductHuntPosts, p.Product.ID)
}

func (p *Post) SourceUIDs() []activitytypes.TypedUID {
	return p.SourceIDs
}

func (p *Post) Title() string {
	return p.Product.Name
}

func (p *Post) Body() string {
	if p.Product.Description != "" {
		return p.Product.Description
	}
	return p.Product.Tagline
}

func (p *Post) URL() string {
	if p.Product.URL != "" {
		return p.Product.URL
	}
	return fmt.Sprintf("https://www.producthunt.com/posts/%s", p.Product.Slug)
}

func (p *Post) ImageURL() string {
	if len(p.Product.Media) > 0 && p.Product.Media[0] != nil {
		return p.Product.Media[0].URL
	}
	if p.Product.Thumbnail != nil && p.Product.Thumbnail.URL != "" {
		return p.Product.Thumbnail.URL
	}
	return ""
}

func (p *Post) UpvotesCount() int {
	return p.Product.VotesCount
}

func (p *Post) DownvotesCount() int {
	return -1
}

func (p *Post) CommentsCount() int {
	return p.Product.CommentsCount
}

func (p *Post) AmplificationCount() int {
	return -1
}

func (p *Post) SocialScore() float64 {
	upvotes := float64(p.UpvotesCount())
	comments := float64(p.CommentsCount())

	scoreWeight := 0.7
	commentsWeight := 0.3

	maxUpvotes := 5000.0

	return (providers.NormSocialScore(upvotes, maxUpvotes) * scoreWeight) +
		(providers.NormSocialScore(comments, maxUpvotes) * commentsWeight)
}

func (p *Post) CreatedAt() time.Time {
	return p.Product.CreatedAt
}

func (s *SourcePosts) Initialize(logger *zerolog.Logger, config *sourcetypes.ProviderConfig) error {
	s.client = NewClient(config.ProductHuntAPIToken, logger)
	s.logger = logger

	return nil
}

func (s *SourcePosts) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	s.fetchProductHuntPosts(ctx, since, feed, errs)
}

func (s *SourcePosts) fetchProductHuntPosts(ctx context.Context, _ activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
	order := PostOrderVotes
	if s.FeedName == "new" {
		order = PostOrderNewest
	}

	timePeriod := TimePeriodToday
	limit := 50
	products, err := s.client.FetchPosts(ctx, order, limit, timePeriod)
	if err != nil {
		errs <- fmt.Errorf("fetch posts: %v", err)
		return
	}

	s.logger.Debug().
		Int("products_count", len(products)).
		Str("order", order.String()).
		Str("time_period", timePeriod.String()).
		Str("feed_name", s.FeedName).
		Int("limit", limit).
		Msg("Fetched products")

	if len(products) == 0 {
		return
	}

	for _, product := range products {
		post := &Post{
			Product:   product,
			SourceIDs: []activitytypes.TypedUID{s.UID()},
		}
		feed <- post
	}
}

func (s *SourcePosts) MarshalJSON() ([]byte, error) {
	type Alias SourcePosts
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  TypeProductHuntPosts,
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
