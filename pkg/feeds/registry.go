package feeds

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/sources"
	activities "github.com/glanceapp/glance/pkg/sources/activities/types"
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
}

type feedStore interface {
	Upsert(ctx context.Context, feed Feed) error
	Remove(ctx context.Context, uid string) error
	List(ctx context.Context) ([]*Feed, error)
	GetByID(ctx context.Context, uid string) (*Feed, error)
}

type summarizer interface {
	SummarizeMany(ctx context.Context, activities []*activities.DecoratedActivity, query string) (*activities.ActivitiesSummary, error)
}

func NewRegistry(store feedStore, sourceExecutor *sources.Executor, sourceRegistry *sources.Registry, summarizer summarizer) *Registry {
	return &Registry{store: store, sourceExecutor: sourceExecutor, sourceRegistry: sourceRegistry, summarizer: summarizer}
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
	// Summary is a cached overview of the most relevant recent activity on the feed.
	Summary activities.ActivitiesSummary

	CreatedAt time.Time
	UpdatedAt time.Time
}

type FeedHighlight struct {
	// Content is a short text summarizing the highlight.
	Content string
	// QuoteActivityIDs source of information for the highlight.
	QuoteActivityIDs []string
}

type CreateFeedRequest struct {
	Name       string
	Icon       string
	Query      string
	SourceUIDs []activities.TypedUID
	UserID     string
}

func (r *Registry) Create(ctx context.Context, req CreateFeedRequest) (*Feed, error) {
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
		Summary:    activities.ActivitiesSummary{},
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

func (r *Registry) Update(ctx context.Context, request Feed) error {
	feed, err := r.store.GetByID(ctx, request.ID)
	if err != nil || feed.UserID != request.UserID {
		return errors.New("feed not found")
	}

	err = r.executeAndUpsert(ctx, request)
	if err != nil {
		return fmt.Errorf("execute and upsert feed: %w", err)
	}

	return nil
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

// Summary returns the overview of the recent relevant activities
// If the userID is provided, an optional queryOverride can be provided as a request-scoped override parameter.
func (r *Registry) Summary(ctx context.Context, feedID, userID, queryOverride string) (*activities.ActivitiesSummary, error) {
	feed, err := r.store.GetByID(ctx, feedID)
	if err != nil {
		return nil, errors.New("feed not found")
	}

	// TODO(config): Move to env config struct
	refreshDuration := 1 * time.Hour
	now := time.Now()
	if now.Sub(feed.Summary.CreatedAt) >= refreshDuration {
		summary, err := r.summarize(ctx, feed.Query, feed.SourceUIDs)
		if err != nil {
			return nil, fmt.Errorf("summarize: %w", err)
		}

		feed.Summary = *summary
		err = r.store.Upsert(ctx, *feed)
		if err != nil {
			return nil, fmt.Errorf("upsert feed: %w", err)
		}
	}

	if userID == "" && queryOverride != "" {
		return nil, ErrAuthUsersOnly
	}

	if queryOverride != "" && queryOverride != feed.Query {
		summary, err := r.summarize(ctx, queryOverride, feed.SourceUIDs)
		if err != nil {
			return nil, fmt.Errorf("summarize: %w", err)
		}

		return summary, nil
	}

	return &feed.Summary, nil
}

func (r *Registry) summarize(ctx context.Context, query string, sourceUIDs []activities.TypedUID) (*activities.ActivitiesSummary, error) {
	limit := 20
	acts, err := r.sourceExecutor.Search(ctx, query, sourceUIDs, 0.0, limit, activities.SortBySimilarity)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if len(acts) < limit {
		// Not enough activities to summarize
		return nil, ErrInsufficientActivity
	}

	summary, err := r.summarizer.SummarizeMany(ctx, acts, query)
	if err != nil {
		return nil, fmt.Errorf("summarize many: %w", err)
	}

	return summary, nil
}

func (r *Registry) Activities(ctx context.Context, feedID, userID string, sortBy activities.SortBy, limit int, queryOverride string) ([]*activities.DecoratedActivity, error) {
	feed, err := r.store.GetByID(ctx, feedID)
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", err)
	}

	// Public feeds can be accessed by anyone (even non-authenticated user)
	if feed.UserID != userID && !feed.Public {
		return nil, errors.New("feed not found")
	}

	if userID == "" && queryOverride != "" {
		return nil, ErrAuthUsersOnly
	}

	// TODO(optimisation): Cache query embeddings
	acts, err := r.sourceExecutor.Search(ctx, feed.Query, feed.SourceUIDs, 0.0, limit, sortBy)
	if err != nil {
		return nil, fmt.Errorf("search activities: %w", err)
	}

	return acts, nil
}
