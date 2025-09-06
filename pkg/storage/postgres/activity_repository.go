package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/lib"

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

	// Get existing update count if activity exists
	existingUpdateCount, err := r.db.Client().Activity.Query().
		Where(entactivity.ID(activity.Activity.UID().String())).
		Select(entactivity.FieldUpdateCount).
		Int(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("get existing update count: %w", err)
	}

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
		SetUpdateCount(existingUpdateCount + 1).
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
		out, err := activityFromEnt(a, 0)
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

func (r *ActivityRepository) Search(req types.SearchRequest) (*types.SearchResult, error) {
	ctx := context.Background()

	// Build the base query for both count and data
	query := r.db.Client().Activity.Query()

	if len(req.SourceUIDs) > 0 {
		sourceUIDs := make([]string, len(req.SourceUIDs))
		for i, uid := range req.SourceUIDs {
			sourceUIDs[i] = uid.String()
		}
		query = query.Where(entactivity.SourceUIDIn(sourceUIDs...))
	}

	if len(req.ActivityUIDs) > 0 {
		activityUIDs := make([]string, len(req.ActivityUIDs))
		for i, uid := range req.ActivityUIDs {
			activityUIDs[i] = uid.String()
		}
		query = query.Where(entactivity.IDIn(activityUIDs...))
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

	if req.Cursor != "" {
		cur, err := deserializeCursor(req.Cursor)
		if err != nil {
			return nil, fmt.Errorf("deserialize cursor: %w", err)
		}
		cursorTime := time.Time(cur.Timestamp)
		cursorID := cur.ID

		if req.SortBy == types.SortByDate {
			// For date sort, filter activities older than the cursor
			query = query.Where(func(s *sql.Selector) {
				s.Where(
					sql.Or(
						// Either the timestamp is less than the cursor
						sql.LT(s.C(entactivity.FieldCreatedAt), cursorTime),
						// Or the timestamp is equal and the ID is different (for ties)
						sql.And(
							sql.EQ(s.C(entactivity.FieldCreatedAt), cursorTime),
							sql.LT(s.C(entactivity.FieldID), cursorID),
						),
					),
				)
			})
		} else {
			return nil, fmt.Errorf("pagination is not supported for sorting by %s", req.SortBy)
		}
	}

	if req.Limit > 0 {
		// Fetch one more to determine if there are more results
		query = query.Limit(req.Limit + 1)
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

	// Check if there are more results
	hasMore := false
	if len(rows) > req.Limit {
		hasMore = true
		rows = rows[:req.Limit] // Remove the extra item
	}

	result := make([]*types.DecoratedActivity, len(rows))
	for i, a := range rows {
		res, err := activityFromEnt(&a.Activity, float32(a.Similarity))
		if err != nil {
			return nil, fmt.Errorf("deserialize db activity: %w", err)
		}
		result[i] = res
	}

	var nextCursor string
	if hasMore && len(result) > 0 {
		lastActivity := result[len(result)-1]

		nextCur := cursor{
			Timestamp: cursorTimestamp(lastActivity.Activity.CreatedAt()),
			ID:        lastActivity.Activity.UID().String(),
		}

		encodedNextCur, err := serializeCursor(nextCur)
		if err != nil {
			return nil, fmt.Errorf("serialize cursor: %w", err)
		}
		nextCursor = encodedNextCur
	}

	return &types.SearchResult{
		Activities: result,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

type cursorTimestamp time.Time

func (ct cursorTimestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(ct).Format(time.RFC3339))
}

func (ct *cursorTimestamp) UnmarshalJSON(data []byte) error {
	var timestampStr string
	if err := json.Unmarshal(data, &timestampStr); err != nil {
		return err
	}

	t, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return fmt.Errorf("parse timestamp: %w", err)
	}

	*ct = cursorTimestamp(t)
	return nil
}

type cursor struct {
	Timestamp cursorTimestamp `json:"timestamp" validate:"required"`
	ID        string          `json:"id" validate:"required"`
}

func serializeCursor(cur cursor) (string, error) {
	cursorJson, err := json.Marshal(cur)
	if err != nil {
		return "", fmt.Errorf("marshal cursor: %w", err)
	}

	return base64.StdEncoding.EncodeToString(cursorJson), nil
}

func deserializeCursor(input string) (cursor, error) {
	cursorBytes, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return cursor{}, fmt.Errorf("decode cursor: %w", err)
	}

	var cur cursor
	err = json.Unmarshal(cursorBytes, &cur)
	if err != nil {
		return cursor{}, fmt.Errorf("unmarshal cursor: %w", err)
	}

	err = lib.ValidateStruct(cur)
	if err != nil {
		return cursor{}, fmt.Errorf("validate cursor: %w", err)
	}

	return cur, nil
}

func activityFromEnt(in *ent.Activity, similarity float32) (*types.DecoratedActivity, error) {
	act, err := activities.NewActivity(in.SourceType)
	if err != nil {
		return nil, fmt.Errorf("new activity: %w", err)
	}

	err = act.UnmarshalJSON([]byte(in.RawJSON))
	if err != nil {
		return nil, fmt.Errorf("unmarshal activity: %w", err)
	}

	return &types.DecoratedActivity{
		Activity:   act,
		Embedding:  in.Embedding.Slice(),
		Similarity: similarity,
		Summary: &types.ActivitySummary{
			ShortSummary: in.ShortSummary,
			FullSummary:  in.FullSummary,
		},
	}, nil
}
