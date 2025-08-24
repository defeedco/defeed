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

	CreatedAt time.Time
	UpdatedAt time.Time

	// TODO: Treat as a separate resource in the future (with its own table/store)
	// Summaries are cached activity summaries, for each unique FeedParameters combination.
	Summaries []FeedSummary
}

type FeedSummary struct {
	Parameters FeedParameters
	Summary    *activities.ActivitiesSummary
}

type FeedHighlight struct {
	// Content is a short text summarizing the highlight.
	Content string
	// QuoteActivityIDs source of information for the highlight.
	QuoteActivityIDs []string
}

type FeedParameters struct {
	FeedID     string
	UserID     string
	Query      string
	SortBy     activities.SortBy
	SourceUIDs []activities.TypedUID
}

func (p FeedParameters) Equal(p1 FeedParameters) bool {
	sourcesEqual := len(p.SourceUIDs) == len(p1.SourceUIDs)
	for i := range p.SourceUIDs {
		if p.SourceUIDs[i].String() != p1.SourceUIDs[i].String() {
			sourcesEqual = false
			break
		}
	}
	return sourcesEqual && p.Query == p1.Query && p.SortBy == p1.SortBy && p.UserID == p1.UserID
}

type Registry struct {
	store          feedStore
	sourceExecutor *sources.Executor
	sourceRegistry *sources.Registry
	summarizer     summarizer
}

type feedStore interface {
	Upsert(ctx context.Context, feed Feed) error
	Remove(ctx context.Context, uid string) error
	ListByUserID(ctx context.Context, userID string) ([]*Feed, error)
	GetByID(ctx context.Context, uid string) (*Feed, error)
}

type summarizer interface {
	SummarizeMany(ctx context.Context, activities []*activities.DecoratedActivity, query string) (*activities.ActivitiesSummary, error)
}

func NewRegistry(store feedStore, sourceExecutor *sources.Executor, summarizer summarizer) *Registry {
	return &Registry{store: store, sourceExecutor: sourceExecutor, summarizer: summarizer}
}

type CreateFeedRequest struct {
	Name       string
	Icon       string
	Query      string
	SourceUIDs []activities.TypedUID
	UserID     string
}

func (r *Registry) Create(ctx context.Context, request CreateFeedRequest) (*Feed, error) {
	feed := Feed{
		ID:         uuid.New().String(),
		Name:       request.Name,
		Icon:       request.Icon,
		Query:      request.Query,
		SourceUIDs: request.SourceUIDs,
		UserID:     request.UserID,
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
			return fmt.Errorf("find source: %w", err)
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

	// TODO: Remove the source from executor if no other feeds are using it
	return r.store.Remove(ctx, uid)
}

func (r *Registry) ListByUserID(ctx context.Context, userID string) ([]*Feed, error) {
	return r.store.ListByUserID(ctx, userID)
}

func (r *Registry) Summary(ctx context.Context, req FeedParameters) (*FeedSummary, error) {
	feed, err := r.store.GetByID(ctx, req.FeedID)
	if err != nil {
		return nil, errors.New("feed not found")
	}

	var summary *FeedSummary
	for _, s := range feed.Summaries {
		if s.Parameters.Equal(req) {
			summary = &s
			break
		}
	}

	// No summary exists for given parameters, compute and cache it
	if summary == nil {
		summary, err = r.summarize(ctx, req)
		if err != nil {
			return nil, err
		}

		// Store updated summary cache
		feed.Summaries = append(feed.Summaries, *summary)
		err = r.store.Upsert(ctx, *feed)
		if err != nil {
			return nil, fmt.Errorf("summarize activities: %w", err)
		}
	}

	return summary, nil
}

func (r *Registry) summarize(ctx context.Context, req FeedParameters) (*FeedSummary, error) {
	activities, err := r.sourceExecutor.Search(ctx, req.Query, req.SourceUIDs, 0.0, 20, req.SortBy)
	if err != nil {
		return nil, err
	}
	summary, err := r.summarizer.SummarizeMany(ctx, activities, req.Query)
	if err != nil {
		return nil, fmt.Errorf("summarize activities: %w", err)
	}
	return &FeedSummary{
		Parameters: req,
		Summary:    summary,
	}, nil
}

func (r *Registry) Activities(ctx context.Context, req FeedParameters) ([]*activities.DecoratedActivity, error) {
	feed, err := r.store.GetByID(ctx, req.FeedID)
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", err)
	}

	if feed.UserID != req.UserID {
		return nil, errors.New("feed not found")
	}

	activities, err := r.sourceExecutor.Search(ctx, req.Query, req.SourceUIDs, 0.0, 20, req.SortBy)
	if err != nil {
		return nil, fmt.Errorf("search activities: %w", err)
	}

	return activities, nil
}
