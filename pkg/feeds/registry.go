package feeds

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/glanceapp/glance/pkg/sources"
	activities "github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/glanceapp/glance/pkg/sources/nlp"
	"github.com/google/uuid"
)

// ErrAuthUsersOnly is used when an action can't be performed without authentication.
// TODO(subscription): Change to "ErrPayingUsersOnly" once we have subscription plans.
var ErrAuthUsersOnly = errors.New("query override supported for authenticated users only")

// ErrInsufficientActivity indicates there is not enough activity to create a summary.
var ErrInsufficientActivity = errors.New("insufficient activity to summarize")

type Registry struct {
	store          feedStore
	sourceExecutor *sources.Executor
	sourceRegistry *sources.Registry
	summarizer     summarizer
	queryRewriter  queryRewriter
	config         *Config
}

type queryRewriter interface {
	RewriteToTopics(ctx context.Context, originalQuery string) ([]*nlp.TopicQueryGroup, error)
}

type feedStore interface {
	Upsert(ctx context.Context, feed Feed) error
	Remove(ctx context.Context, uid string) error
	List(ctx context.Context) ([]*Feed, error)
	GetByID(ctx context.Context, uid string) (*Feed, error)
}

type summarizer interface {
	SummarizeTopic(ctx context.Context, topic *nlp.TopicQueryGroup, activities []*activities.DecoratedActivity) (string, error)
}

func NewRegistry(store feedStore, sourceExecutor *sources.Executor, sourceRegistry *sources.Registry, summarizer summarizer, queryRewriter queryRewriter, config *Config) *Registry {
	return &Registry{
		store:          store,
		sourceExecutor: sourceExecutor,
		sourceRegistry: sourceRegistry,
		summarizer:     summarizer,
		queryRewriter:  queryRewriter,
		config:         config,
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
	SourceUIDs []activities.TypedUID
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
	SourceUIDs []activities.TypedUID
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
	SourceUIDs []activities.TypedUID
}

func (r *Registry) Update(ctx context.Context, req UpdateRequest) (*Feed, error) {
	feed, err := r.store.GetByID(ctx, req.ID)
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
	err := r.store.Upsert(ctx, feed)
	if err != nil {
		return fmt.Errorf("upsert feed: %w", err)
	}

	for _, sourceUID := range feed.SourceUIDs {
		source, err := r.sourceRegistry.FindByUID(ctx, sourceUID)
		if err != nil {
			return fmt.Errorf("find source %s: %w", sourceUID, err)
		}

		err = r.sourceExecutor.Add(source)
		if err != nil {
			return fmt.Errorf("add source to executor: %w", err)
		}
	}

	return nil
}

func (r *Registry) Remove(ctx context.Context, uid string, userID string) error {
	feed, err := r.store.GetByID(ctx, uid)
	if err != nil || feed.UserID != userID {
		return errors.New("feed not found")
	}

	// TODO(optimisation): Remove the source from executor if no other feeds are using it
	return r.store.Remove(ctx, uid)
}

// ListByUserID returns both the feeds that the user owns and public ones.
// If userID is empty, only public feeds are returned.
func (r *Registry) ListByUserID(ctx context.Context, userID string) ([]*Feed, error) {
	feeds, err := r.store.List(ctx)
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
	Results []*activities.DecoratedActivity
	Topics  []*Topic
}

type Topic struct {
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Queries     []string `json:"queries"`
	ActivityIDs []string `json:"activityIds"`
}

func (r *Registry) Activities(
	ctx context.Context,
	feedID string,
	userID string,
	sortBy activities.SortBy,
	limit int,
	queryOverride string,
	period activities.Period,
) (*ActivitiesResponse, error) {
	feed, err := r.store.GetByID(ctx, feedID)
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

	topicQueryGroups, err := r.queryRewriter.RewriteToTopics(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("rewrite query to topics: %w", err)
	}

	acts, activityToTopic, err := r.searchByTopicQueryGroups(ctx, feed.SourceUIDs, topicQueryGroups, sortBy, period, limit)
	if err != nil {
		return nil, fmt.Errorf("search by topic query groups: %w", err)
	}

	var topicToSummary map[string]string
	if r.config.SummarizeTopics {
		topicToSummary, err = r.summarizeTopics(ctx, topicQueryGroups, acts, activityToTopic)
		if err != nil {
			return nil, fmt.Errorf("summarize topics: %w", err)
		}
	}

	topics := make([]*Topic, len(topicQueryGroups))
	for i, topicGroup := range topicQueryGroups {
		activityIDs := make([]string, 0)
		for actID, topic := range activityToTopic {
			if topic == topicGroup.Topic {
				activityIDs = append(activityIDs, actID)
			}
		}

		// Allow empty summary
		summary := topicToSummary[topicGroup.Topic]

		topics[i] = &Topic{
			Title:       topicGroup.Topic,
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
	sourceUIDs []activities.TypedUID,
	topics []*nlp.TopicQueryGroup,
	sortBy activities.SortBy,
	period activities.Period,
	limit int,
) ([]*activities.DecoratedActivity, map[string]string, error) {
	actsByGroupByQuery := make([][][]*activities.DecoratedActivity, len(topics))

	// Calculate limit per topic to ensure we don't exceed the total limit
	limitPerTopic := limit / len(topics)
	if limitPerTopic == 0 {
		limitPerTopic = 1
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(-1) // no limit

	for ti, topic := range topics {
		actsByGroupByQuery[ti] = make([][]*activities.DecoratedActivity, len(topic.Queries))
		for qi, query := range topic.Queries {
			g.Go(func() error {
				// TODO: Set min similarity filter?
				acts, err := r.sourceExecutor.Search(gctx, query, sourceUIDs, 0.0, limitPerTopic, sortBy, period)
				if err != nil {
					return fmt.Errorf("search activities for topic %s: %w", topic.Topic, err)
				}

				actsByGroupByQuery[ti][qi] = acts

				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return nil, nil, fmt.Errorf("wait search: %w", err)
	}

	seenActs := make(map[string]bool)
	activityToTopic := make(map[string]string)
	acts := make([]*activities.DecoratedActivity, 0)
	for ti, topicGroup := range actsByGroupByQuery {
		for _, queryGroup := range topicGroup {
			for _, act := range queryGroup {
				if seenActs[act.Activity.UID().String()] {
					continue
				}

				activityToTopic[act.Activity.UID().String()] = topics[ti].Topic
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
	topics []*nlp.TopicQueryGroup,
	allActivities []*activities.DecoratedActivity,
	activityToTopic map[string]string,
) (map[string]string, error) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5)

	indexedSummaries := make([]string, len(topics))
	for ti, topic := range topics {
		topicActs := make([]*activities.DecoratedActivity, 0)
		for actID, actTopic := range activityToTopic {
			if topic.Topic == actTopic {
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
			summary, err := r.summarizer.SummarizeTopic(gctx, topic, topicActs)
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
		topicToSummary[topics[ti].Topic] = summary
	}

	return topicToSummary, nil
}
