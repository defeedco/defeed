package feeds

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources/activities"
	sourcetypes "github.com/defeedco/defeed/pkg/sources/types"

	"golang.org/x/sync/errgroup"

	"github.com/defeedco/defeed/pkg/sources"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/defeedco/defeed/pkg/sources/nlp"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ErrAuthUsersOnly is used when an action can't be performed without authentication.
// TODO(subscription): Change to "ErrPayingUsersOnly" once we have subscription plans.
var ErrAuthUsersOnly = errors.New("query override supported for authenticated users only")

type Registry struct {
	feedRepository   feedStore
	sourceScheduler  *sources.Scheduler
	sourceRegistry   sourceRegistry
	activityRegistry *activities.Registry
	summarizer       summarizer
	queryRewriter    *nlp.QueryRewriter
	config           *Config
	cache            *lib.Cache
	logger           *zerolog.Logger
}

type feedStore interface {
	Upsert(ctx context.Context, feed Feed) error
	Remove(ctx context.Context, uid string) error
	List(ctx context.Context) ([]*Feed, error)
	GetByID(ctx context.Context, uid string) (*Feed, error)
}

type summarizer interface {
	SummarizeTopic(ctx context.Context, topic *nlp.TopicQueryGroup, activities []*activitytypes.DecoratedActivity) (string, error)
}

type sourceRegistry interface {
	FindByUID(ctx context.Context, uid activitytypes.TypedUID) (sourcetypes.Source, error)
}

func NewRegistry(
	feedRepository feedStore,
	sourceScheduler *sources.Scheduler,
	sourceRegistry sourceRegistry,
	activityRegistry *activities.Registry,
	summarizer summarizer,
	queryRewriter *nlp.QueryRewriter,
	config *Config,
	logger *zerolog.Logger,
) *Registry {
	return &Registry{
		feedRepository:   feedRepository,
		sourceScheduler:  sourceScheduler,
		sourceRegistry:   sourceRegistry,
		activityRegistry: activityRegistry,
		summarizer:       summarizer,
		queryRewriter:    queryRewriter,
		config:           config,
		// TODO: be smarter about when to revalidate summaries and or queries (e.g. when the activities are sufficiently different)
		cache:  lib.NewCache(2*time.Hour, logger),
		logger: logger,
	}
}

type Feed struct {
	ID   string
	Name string
	// Icon is a string of emoji characters.
	Icon string
	// Query is a semantic search query.
	Query string
	// SourceUIDs is a list of sources where activities are pulled from.
	SourceUIDs []activitytypes.TypedUID
	// UserID is the user who owns the feed.
	UserID string
	// Public is true if any user can access the feed.
	Public bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

type FeedHighlight struct {
	// Content is a short text summarizing the highlight.
	Content string
	// QuoteActivityIDs source of information for the highlight.
	QuoteActivityIDs []string
}

type CreateRequest struct {
	Name       string
	Icon       string
	Query      string
	SourceUIDs []activitytypes.TypedUID
	UserID     string
}

func (r *Registry) Create(ctx context.Context, req CreateRequest) (*Feed, error) {
	// TODO(validation): Add more comprehensive validation using "validate" go field tags
	if req.UserID == "" {
		return nil, errors.New("user ID is required")
	}

	feed := Feed{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Icon:       req.Icon,
		Query:      req.Query,
		SourceUIDs: req.SourceUIDs,
		UserID:     req.UserID,
		Public:     false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err := r.executeAndUpsert(ctx, feed)
	if err != nil {
		return nil, fmt.Errorf("execute and upsert feed: %w", err)
	}

	return &feed, nil
}

type UpdateRequest struct {
	ID         string
	UserID     string
	Name       string
	Icon       string
	Query      string
	SourceUIDs []activitytypes.TypedUID
}

func (r *Registry) Update(ctx context.Context, req UpdateRequest) (*Feed, error) {
	feed, err := r.feedRepository.GetByID(ctx, req.ID)
	if err != nil || feed.UserID != req.UserID {
		return nil, errors.New("feed not found")
	}

	// Update the customizable fields,
	// but preserve the internal state (Public, Summary,...)
	feed.Name = req.Name
	feed.Icon = req.Icon
	feed.Query = req.Query
	feed.SourceUIDs = req.SourceUIDs
	feed.UpdatedAt = time.Now()

	err = r.executeAndUpsert(ctx, *feed)
	if err != nil {
		return nil, fmt.Errorf("execute and upsert feed: %w", err)
	}

	return feed, nil
}

func (r *Registry) executeAndUpsert(ctx context.Context, feed Feed) error {
	err := r.feedRepository.Upsert(ctx, feed)
	if err != nil {
		return fmt.Errorf("upsert feed: %w", err)
	}

	for _, sourceUID := range feed.SourceUIDs {
		source, err := r.sourceRegistry.FindByUID(ctx, sourceUID)
		if err != nil {
			return fmt.Errorf("find source %s: %w", sourceUID, err)
		}

		err = r.sourceScheduler.Add(source)
		if err != nil {
			return fmt.Errorf("add source to executor: %w", err)
		}
	}

	return nil
}

func (r *Registry) Remove(ctx context.Context, uid string, userID string) error {
	feed, err := r.feedRepository.GetByID(ctx, uid)
	if err != nil || feed.UserID != userID {
		return errors.New("feed not found")
	}

	// TODO(optimisation): Remove the source from executor if no other feeds are using it
	return r.feedRepository.Remove(ctx, uid)
}

// ListByUserID returns both the feeds that the user owns and public ones.
// If userID is empty, only public feeds are returned.
func (r *Registry) ListByUserID(ctx context.Context, userID string) ([]*Feed, error) {
	feeds, err := r.feedRepository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list feeds: %w", err)
	}

	authorizedFeeds := make([]*Feed, 0)
	for _, feed := range feeds {
		if feed.UserID == userID || feed.Public {
			authorizedFeeds = append(authorizedFeeds, feed)
		}
	}

	return authorizedFeeds, nil
}

type ActivitiesResponse struct {
	Results []*activitytypes.DecoratedActivity
	Topics  []*Topic
}

type Topic struct {
	Title       string
	Emoji       string
	Summary     string
	Queries     []string
	ActivityIDs []string
}

func (r *Registry) Activities(
	ctx context.Context,
	feedID string,
	userID string,
	sortBy activitytypes.SortBy,
	limit int,
	queryOverride string,
	period activitytypes.Period,
) (*ActivitiesResponse, error) {
	feed, err := r.feedRepository.GetByID(ctx, feedID)
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", err)
	}

	// Public feeds can be accessed by anyone (even non-authenticated user)
	if feed.UserID != userID && !feed.Public {
		return nil, errors.New("feed not found")
	}

	query := feed.Query
	if queryOverride != "" {
		if userID == "" {
			return nil, ErrAuthUsersOnly
		}
		query = queryOverride
	}

	if query != "" {
		// Only if user provides the query, we can rewrite it to sub-queries and return results in topics.
		return r.searchByQuery(ctx, feed.SourceUIDs, query, sortBy, period, limit)
	}

	result, err := r.activityRegistry.Search(ctx, activities.SearchRequest{
		SourceUIDs: feed.SourceUIDs,
		SortBy:     sortBy,
		Period:     period,
		Limit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search activities: %w", err)
	}

	return &ActivitiesResponse{
		Results: result.Activities,
		Topics:  nil,
	}, nil
}

func (r *Registry) searchByQuery(
	ctx context.Context,
	sourceUIDs []activitytypes.TypedUID,
	query string,
	sortBy activitytypes.SortBy,
	period activitytypes.Period,
	limit int,
) (*ActivitiesResponse, error) {
	// For now list active sources from the scheduler instead of the source registry,
	// since the source registry is fetching some sources from the 3rd party APIs and may hit rate limits.
	feedSources, err := r.sourceScheduler.List(sources.ListRequest{
		SourceUIDs: sourceUIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}

	topicQueryGroups, err := r.queryRewriter.RewriteToTopics(ctx, nlp.RewriteRequest{
		Query:   query,
		Sources: feedSources,
	})
	if err != nil {
		return nil, fmt.Errorf("rewrite query to topics: %w", err)
	}

	acts, activityToTopic, err := r.searchByTopicQueryGroups(ctx, sourceUIDs, topicQueryGroups, sortBy, period, limit)
	if err != nil {
		return nil, fmt.Errorf("search by topic query groups: %w", err)
	}

	// Note: topic summaries are disabled for now,
	// since they seem to add unecessary noise in the UI
	// and noticably increase the latency of the request.
	var topicToSummary map[string]string
	if r.config.SummarizeTopics {
		topicToSummary, err = r.summarizeTopics(ctx, period, topicQueryGroups, acts, activityToTopic)
		if err != nil {
			return nil, fmt.Errorf("summarize topics: %w", err)
		}
	}

	topics := make([]*Topic, len(topicQueryGroups))
	for i, topicGroup := range topicQueryGroups {
		activityIDs := make([]string, 0)
		for actID, topic := range activityToTopic {
			if topic == topicGroup.Name {
				activityIDs = append(activityIDs, actID)
			}
		}

		// Allow empty summary
		summary := topicToSummary[topicGroup.Name]

		topics[i] = &Topic{
			Title:       topicGroup.Name,
			Emoji:       topicGroup.Emoji,
			Queries:     topicGroup.Queries,
			ActivityIDs: activityIDs,
			Summary:     summary,
		}
	}

	return &ActivitiesResponse{
		Results: acts,
		Topics:  topics,
	}, nil
}

func (r *Registry) searchByTopicQueryGroups(
	ctx context.Context,
	sourceUIDs []activitytypes.TypedUID,
	topics []*nlp.TopicQueryGroup,
	sortBy activitytypes.SortBy,
	period activitytypes.Period,
	limit int,
) ([]*activitytypes.DecoratedActivity, map[string]string, error) {
	actsByGroupByQuery := make([][][]*activitytypes.DecoratedActivity, len(topics))

	// Calculate limit per topic to ensure we don't exceed the total limit
	limitPerTopic := limit / len(topics)
	if limitPerTopic == 0 {
		limitPerTopic = 1
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(-1) // no limit

	for ti, topic := range topics {
		actsByGroupByQuery[ti] = make([][]*activitytypes.DecoratedActivity, len(topic.Queries))
		for qi, query := range topic.Queries {
			g.Go(func() error {
				res, err := r.activityRegistry.Search(gctx, activities.SearchRequest{
					Query:      query,
					SourceUIDs: sourceUIDs,
					// TODO: Set min similarity filter?
					MinSimilarity: 0.0,
					Limit:         limitPerTopic,
					SortBy:        sortBy,
					Period:        period,
				})
				if err != nil {
					return fmt.Errorf("search activities for topic %s: %w", topic.Name, err)
				}

				actsByGroupByQuery[ti][qi] = res.Activities

				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return nil, nil, fmt.Errorf("wait search: %w", err)
	}

	seenActs := make(map[string]bool)
	activityToTopic := make(map[string]string)
	acts := make([]*activitytypes.DecoratedActivity, 0)
	for ti, topicGroup := range actsByGroupByQuery {
		for _, queryGroup := range topicGroup {
			for _, act := range queryGroup {
				if seenActs[act.Activity.UID().String()] {
					continue
				}

				activityToTopic[act.Activity.UID().String()] = topics[ti].Name
				seenActs[act.Activity.UID().String()] = true
				acts = append(acts, act)
			}
		}
	}

	sort.Slice(acts, func(i, j int) bool {
		return acts[i].Similarity > acts[j].Similarity
	})

	return acts, activityToTopic, nil
}

func (r *Registry) summarizeTopics(
	ctx context.Context,
	period activitytypes.Period,
	topics []*nlp.TopicQueryGroup,
	allActivities []*activitytypes.DecoratedActivity,
	activityToTopic map[string]string,
) (map[string]string, error) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(-1) // no limit

	indexedSummaries := make([]string, len(topics))
	for ti, topic := range topics {
		topicActs := make([]*activitytypes.DecoratedActivity, 0)
		for actID, actTopic := range activityToTopic {
			if topic.Name == actTopic {
			actLoop:
				for _, act := range allActivities {
					if act.Activity.UID().String() == actID {
						topicActs = append(topicActs, act)
						break actLoop
					}
				}
			}
		}
		g.Go(func() error {
			summary, err := r.summarizeTopicWithCache(gctx, period, topic, topicActs)
			if err != nil {
				return fmt.Errorf("summarize topic activities: %w", err)
			}

			indexedSummaries[ti] = summary

			return nil

		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("wait summarize: %w", err)
	}

	topicToSummary := make(map[string]string)
	for ti, summary := range indexedSummaries {
		topicToSummary[topics[ti].Name] = summary
	}

	return topicToSummary, nil
}

func (r *Registry) summarizeTopicWithCache(
	ctx context.Context,
	period activitytypes.Period,
	topic *nlp.TopicQueryGroup,
	activities []*activitytypes.DecoratedActivity,
) (string, error) {
	if len(activities) == 0 {
		return "", nil
	}

	cacheKey := fmt.Sprintf("topic_summary:%s:%s", period, topic.Name)

	if cached, found := r.cache.Get(cacheKey); found {
		if summary, ok := cached.(string); ok {
			r.logger.Debug().
				Str("topic", topic.Name).
				Int("activity_count", len(activities)).
				Msg("topic summary cache hit")
			return summary, nil
		}
	}

	summary, err := r.summarizer.SummarizeTopic(ctx, topic, activities)
	if err != nil {
		return "", err
	}

	r.cache.Set(cacheKey, summary)
	r.logger.Debug().
		Str("topic", topic.Name).
		Int("activity_count", len(activities)).
		Msg("topic summary cached")

	return summary, nil
}
