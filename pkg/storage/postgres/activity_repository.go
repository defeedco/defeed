package postgres

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/glanceapp/glance/pkg/sources/activities"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/pgvector/pgvector-go"

	"github.com/glanceapp/glance/pkg/storage/postgres/ent"
	"github.com/glanceapp/glance/pkg/storage/postgres/ent/activity"
)

type ActivityRepository struct {
	db *DB
}

func NewActivityRepository(db *DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) Add(activity *types.DecoratedActivity) error {
	ctx := context.Background()

	rawJson, err := activity.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal activity: %w", err)
	}

	_, err = r.db.Client().Activity.Create().
		SetID(activity.UID()).
		SetUID(activity.UID()).
		SetSourceUID(activity.SourceUID()).
		SetTitle(activity.Title()).
		SetBody(activity.Body()).
		SetURL(activity.URL()).
		SetImageURL(activity.ImageURL()).
		SetCreatedAt(activity.CreatedAt()).
		SetSourceType(activity.SourceType()).
		SetRawJSON(string(rawJson)).
		SetShortSummary(activity.Summary.ShortSummary).
		SetFullSummary(activity.Summary.FullSummary).
		SetEmbedding(pgvector.NewVector(activity.Embedding)).
		Save(ctx)

	return err
}

func (r *ActivityRepository) Remove(uid string) error {
	ctx := context.Background()
	return r.db.Client().Activity.DeleteOneID(uid).Exec(ctx)
}

func (r *ActivityRepository) List() ([]*types.DecoratedActivity, error) {
	ctx := context.Background()

	results, err := r.db.Client().Activity.Query().
		Order(ent.Desc(activity.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*types.DecoratedActivity, len(results))
	for i, a := range results {
		out, err := activityFromEnt(a)
		if err != nil {
			return nil, fmt.Errorf("deserialize activity: %w", err)
		}
		result[i] = out
	}

	return result, nil
}

type activityWithSimilarity struct {
	ent.Activity
	Similarity float64 `sql:"similarity"`
}

func (r *ActivityRepository) Search(req types.SearchRequest) ([]*types.DecoratedActivity, error) {
	ctx := context.Background()

	query := r.db.Client().Activity.Query()

	if len(req.SourceUIDs) > 0 {
		query = query.Where(activity.SourceUIDIn(req.SourceUIDs...))
	}

	query = query.Order(func(s *sql.Selector) {
		var simExpr string
		if len(req.QueryEmbedding) > 0 {
			vector := pgvector.NewVector(req.QueryEmbedding)
			simExpr = fmt.Sprintf("(1 - (embedding <=> '%s'))", vector)
			if req.MinSimilarity > 0 {
				s.Where(sql.GT(simExpr, req.MinSimilarity))
			}
		} else {
			simExpr = "CAST(0 AS float8)"
		}
		s.AppendSelect(sql.As(simExpr, "similarity"))
	})

	if req.Limit > 0 {
		query = query.Limit(req.Limit)
	}

	switch req.SortBy {
	case types.SortBySimilarity:
		if len(req.QueryEmbedding) > 0 {
			query = query.Order(func(s *sql.Selector) {
				s.OrderExpr(sql.Expr("similarity DESC"))
			})
		}
	case types.SortByDate:
		query = query.Order(ent.Desc(activity.FieldCreatedAt))
	}

	fields := []string{
		activity.FieldID,
		activity.FieldUID,
		activity.FieldSourceUID,
		activity.FieldSourceType,
		activity.FieldTitle,
		activity.FieldBody,
		activity.FieldURL,
		activity.FieldImageURL,
		activity.FieldCreatedAt,
		activity.FieldShortSummary,
		activity.FieldFullSummary,
		activity.FieldRawJSON,
		activity.FieldEmbedding,
	}

	var rows []activityWithSimilarity
	err := query.Select(fields...).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("search scan: %w", err)
	}

	result := make([]*types.DecoratedActivity, len(rows))
	for i, a := range rows {
		act, err := activities.NewActivity(a.SourceType)
		if err != nil {
			return nil, fmt.Errorf("new activity: %w", err)
		}
		err = act.UnmarshalJSON([]byte(a.RawJSON))
		if err != nil {
			return nil, fmt.Errorf("unmarshal activity: %w", err)
		}
		result[i] = &types.DecoratedActivity{
			Activity: act,
			Summary: &types.ActivitySummary{
				ShortSummary: a.ShortSummary,
				FullSummary:  a.FullSummary,
			},
			// Embedding:  a.Embedding.Slice(),
			Similarity: float32(a.Similarity),
		}
	}

	return result, nil
}

func activityFromEnt(in *ent.Activity) (*types.DecoratedActivity, error) {
	act, err := activities.NewActivity(in.SourceType)
	if err != nil {
		return nil, fmt.Errorf("new activity: %w", err)
	}

	err = act.UnmarshalJSON([]byte(in.RawJSON))
	if err != nil {
		return nil, fmt.Errorf("unmarshal activity: %w", err)
	}

	return &types.DecoratedActivity{
		Activity: act,
		Summary: &types.ActivitySummary{
			ShortSummary: in.ShortSummary,
			FullSummary:  in.FullSummary,
		},
	}, nil
}
