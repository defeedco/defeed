package postgres

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/glanceapp/glance/pkg/sources/activities"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/pgvector/pgvector-go"

	"github.com/glanceapp/glance/pkg/storage/postgres/ent"
	entactivity "github.com/glanceapp/glance/pkg/storage/postgres/ent/activity"
)

type ActivityRepository struct {
	db *DB
}

func NewActivityRepository(db *DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) Upsert(activity *types.DecoratedActivity) error {
	ctx := context.Background()

	rawJson, err := activity.Activity.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal activity: %w", err)
	}

	err = r.db.Client().Activity.Create().
		SetID(activity.Activity.UID().String()).
		SetUID(activity.Activity.UID().String()).
		SetSourceUID(activity.Activity.SourceUID().String()).
		SetTitle(activity.Activity.Title()).
		SetBody(activity.Activity.Body()).
		SetURL(activity.Activity.URL()).
		SetImageURL(activity.Activity.ImageURL()).
		SetCreatedAt(activity.Activity.CreatedAt()).
		SetSourceType(activity.Activity.SourceUID().Type()).
		SetRawJSON(string(rawJson)).
		SetShortSummary(activity.Summary.ShortSummary).
		SetFullSummary(activity.Summary.FullSummary).
		SetEmbedding(pgvector.NewVector(activity.Embedding)).
		// https://github.com/ent/ent/issues/2494#issuecomment-1182015427
		OnConflictColumns(entactivity.FieldID).
		UpdateNewValues().
		Exec(ctx)

	return err
}

func (r *ActivityRepository) Remove(uid string) error {
	ctx := context.Background()
	return r.db.Client().Activity.DeleteOneID(uid).Exec(ctx)
}

func (r *ActivityRepository) List() ([]*types.DecoratedActivity, error) {
	ctx := context.Background()

	results, err := r.db.Client().Activity.Query().
		Order(ent.Desc(entactivity.FieldCreatedAt)).
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
		sourceUIDs := make([]string, len(req.SourceUIDs))
		for i, uid := range req.SourceUIDs {
			sourceUIDs[i] = uid.String()
		}
		query = query.Where(entactivity.SourceUIDIn(sourceUIDs...))
	}

	// Add time-based filtering based on period
	if req.Period != types.PeriodAll {
		var since time.Time
		now := time.Now()

		switch req.Period {
		case types.PeriodMonth:
			// Start of last month
			since = time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
		case types.PeriodWeek:
			// Start of last week (Monday)
			daysSinceMonday := int(now.Weekday()) - 1
			if daysSinceMonday < 0 {
				daysSinceMonday = 6
			}
			since = now.AddDate(0, 0, -daysSinceMonday-7).Truncate(24 * time.Hour)
		case types.PeriodDay:
			// Start of yesterday
			since = now.AddDate(0, 0, -1).Truncate(24 * time.Hour)
		}

		query = query.Where(entactivity.CreatedAtGTE(since))
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
		} else {
			return nil, fmt.Errorf("sort by similarity requires query embedding parameter")
		}
	case types.SortByDate:
		query = query.Order(ent.Desc(entactivity.FieldCreatedAt))
	}

	fields := []string{
		entactivity.FieldID,
		entactivity.FieldUID,
		entactivity.FieldSourceUID,
		entactivity.FieldSourceType,
		entactivity.FieldTitle,
		entactivity.FieldBody,
		entactivity.FieldURL,
		entactivity.FieldImageURL,
		entactivity.FieldCreatedAt,
		entactivity.FieldShortSummary,
		entactivity.FieldFullSummary,
		entactivity.FieldRawJSON,
		entactivity.FieldEmbedding,
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
			Embedding:  a.Embedding.Slice(),
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
