package postgres

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/defeedco/defeed/pkg/sources/activities/types"

	"github.com/defeedco/defeed/pkg/feeds"
	"github.com/defeedco/defeed/pkg/sources"
	"github.com/defeedco/defeed/pkg/storage/postgres/ent"
	entfeed "github.com/defeedco/defeed/pkg/storage/postgres/ent/feed"
	"github.com/defeedco/defeed/pkg/storage/postgres/ent/predicate"
)

type FeedRepository struct {
	db *DB
}

func NewFeedRepository(db *DB) *FeedRepository {
	return &FeedRepository{db: db}
}

func (r *FeedRepository) Upsert(ctx context.Context, f feeds.Feed) error {

	sourceUIDs := make([]string, len(f.SourceUIDs))
	for i, uid := range f.SourceUIDs {
		sourceUIDs[i] = uid.String()
	}

	err := r.db.Client().Feed.Create().
		SetID(f.ID).
		SetUserID(f.UserID).
		SetName(f.Name).
		SetIcon(f.Icon).
		SetQuery(f.Query).
		SetSourceUids(sourceUIDs).
		SetPublic(f.Public).
		SetUpdatedAt(f.UpdatedAt).
		SetCreatedAt(f.CreatedAt).
		// https://github.com/ent/ent/issues/2494#issuecomment-1182015427
		OnConflictColumns(entfeed.FieldID).
		UpdateNewValues().
		Exec(ctx)

	return err
}

func (r *FeedRepository) Remove(ctx context.Context, uid string) error {
	return r.db.Client().Feed.DeleteOneID(uid).Exec(ctx)
}

func (r *FeedRepository) List(ctx context.Context) ([]*feeds.Feed, error) {
	feedsEnt, err := r.db.Client().Feed.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*feeds.Feed, len(feedsEnt))
	for i, f := range feedsEnt {
		result[i], err = feedFromEnt(f)
		if err != nil {
			return nil, fmt.Errorf("deserialize feed: %w", err)
		}
	}

	return result, nil
}

func (r *FeedRepository) GetByID(ctx context.Context, uid string) (*feeds.Feed, error) {
	f, err := r.db.Client().Feed.Query().Where(entfeed.ID(uid)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("feed not found")
		}
		return nil, err
	}

	return feedFromEnt(f)
}

func (r *FeedRepository) FindBySourceUIDs(ctx context.Context, sourceUIDs []types.TypedUID) ([]*feeds.Feed, error) {
	if len(sourceUIDs) == 0 {
		return []*feeds.Feed{}, nil
	}

	sourceUIDStrings := make([]string, len(sourceUIDs))
	for i, uid := range sourceUIDs {
		sourceUIDStrings[i] = uid.String()
	}

	predicates := make([]predicate.Feed, len(sourceUIDStrings))
	for i, uid := range sourceUIDStrings {
		predicates[i] = func(s *sql.Selector) {
			s.Where(sql.P(func(b *sql.Builder) {
				b.WriteString(entfeed.FieldSourceUids)
				b.WriteString(" @> ")
				b.Arg(fmt.Sprintf(`["%s"]`, uid))
			}))
		}
	}

	feedsEnt, err := r.db.Client().Feed.Query().
		Where(entfeed.Or(predicates...)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*feeds.Feed, len(feedsEnt))
	for i, f := range feedsEnt {
		result[i], err = feedFromEnt(f)
		if err != nil {
			return nil, fmt.Errorf("deserialize feed: %w", err)
		}
	}

	return result, nil
}

func feedFromEnt(in *ent.Feed) (*feeds.Feed, error) {
	sourceUIDs := make([]types.TypedUID, len(in.SourceUids))
	for i, uid := range in.SourceUids {
		typedUID, err := sources.NewTypedUID(uid)
		if err != nil {
			return nil, fmt.Errorf("deserialize source UID: %w", err)
		}
		sourceUIDs[i] = typedUID
	}

	return &feeds.Feed{
		ID:         in.ID,
		UserID:     in.UserID,
		Name:       in.Name,
		Icon:       in.Icon,
		Query:      in.Query,
		SourceUIDs: sourceUIDs,
		CreatedAt:  in.CreatedAt,
		UpdatedAt:  in.UpdatedAt,
		Public:     in.Public,
	}, nil
}
