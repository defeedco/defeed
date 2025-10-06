package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources/activities"
	"github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/defeedco/defeed/pkg/sources/providers"
	"github.com/pgvector/pgvector-go"

	"github.com/defeedco/defeed/pkg/storage/postgres/ent"
	entactivity "github.com/defeedco/defeed/pkg/storage/postgres/ent/activity"
	"github.com/defeedco/defeed/pkg/storage/postgres/ent/predicate"
)

type ActivityRepository struct {
	db *DB
}

func NewActivityRepository(db *DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

type partialActivity struct {
	UpdateCount int      `json:"update_count"`
	SourceUids  []string `json:"source_uids"`
}

func (r *ActivityRepository) Upsert(ctx context.Context, activity *types.DecoratedActivity) error {
	// TODO: Test this
	existingPartialActivities := []partialActivity{}
	err := r.db.Client().Activity.Query().
		Where(entactivity.ID(activity.Activity.UID().String())).
		Select(entactivity.FieldUpdateCount, entactivity.FieldSourceUids).
		Scan(ctx, &existingPartialActivities)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("get existing update count: %w", err)
	}

	existingPartialActivity := partialActivity{
		UpdateCount: 0,
		SourceUids:  []string{},
	}
	if len(existingPartialActivities) == 1 {
		existingPartialActivity = existingPartialActivities[0]
	}

	rawJson, err := activity.Activity.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal activity: %w", err)
	}

	sourceUIDs := existingPartialActivity.SourceUids
	for _, uid := range activity.Activity.SourceUIDs() {
		sourceUIDs = append(sourceUIDs, uid.String())
	}

	// Assume all sources are of the same type.
	var sourceType string
	if len(sourceUIDs) > 0 {
		sourceType = activity.Activity.SourceUIDs()[0].Type()
	}

	qb := r.db.Client().Activity.Create().
		SetID(activity.Activity.UID().String()).
		SetUID(activity.Activity.UID().String()).
		SetSourceUids(sourceUIDs).
		SetTitle(activity.Activity.Title()).
		SetBody(activity.Activity.Body()).
		SetURL(activity.Activity.URL()).
		SetImageURL(activity.Activity.ImageURL()).
		SetCreatedAt(activity.Activity.CreatedAt()).
		SetSourceType(sourceType).
		SetRawJSON(string(rawJson)).
		SetShortSummary(activity.Summary.ShortSummary).
		SetFullSummary(activity.Summary.FullSummary).
		SetSocialScore(activity.Activity.SocialScore()).
		SetUpdateCount(existingPartialActivity.UpdateCount + 1)

	switch len(activity.Embedding) {
	case 1536:
		qb = qb.SetEmbedding1536(pgvector.NewVector(activity.Embedding))
	case 3072:
		qb = qb.SetEmbedding3072(pgvector.NewVector(activity.Embedding))
	case 0:
		// Do nothing
	default:
		return fmt.Errorf("invalid embedding length: %d", len(activity.Embedding))
	}

	err = qb.
		// https://github.com/ent/ent/issues/2494#issuecomment-1182015427
		OnConflictColumns(entactivity.FieldID).
		UpdateNewValues().
		Exec(ctx)

	return err
}

type activityWithSimilarity struct {
	ent.Activity
	Similarity    float64 `sql:"similarity"`
	WeightedScore float64 `sql:"weighted_score"`
}

func (r *ActivityRepository) Search(ctx context.Context, req types.SearchRequest) (*types.SearchResult, error) {
	// Build the base query for both count and data
	query := r.db.Client().Activity.Query()

	if len(req.SourceUIDs) > 0 {
		sourceUIDs := make([]string, len(req.SourceUIDs))
		for i, uid := range req.SourceUIDs {
			sourceUIDs[i] = uid.String()
		}
		predicates := make([]*sql.Predicate, len(sourceUIDs))
		for i, uid := range sourceUIDs {
			predicates[i] = sql.P(func(b *sql.Builder) {
				b.WriteString(entactivity.FieldSourceUids)
				b.WriteString(" @> ")
				b.Arg(fmt.Sprintf(`["%s"]`, uid))
			})
		}
		query = query.Where(func(s *sql.Selector) {
			s.Where(sql.Or(predicates...))
		})
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

	var embeddingField string
	switch len(req.QueryEmbedding) {
	case 1536:
		embeddingField = entactivity.FieldEmbedding1536
	case 3072:
		embeddingField = entactivity.FieldEmbedding3072
	case 0:
		// Do nothing
	default:
		return nil, fmt.Errorf("invalid embedding length: %d", len(req.QueryEmbedding))
	}

	// There might be some unprocessed activities with empty embeddings from incomplete data migrations.
	// Filter them out, or we'll get invalid similarity scores.
	if embeddingField != "" {
		query = query.Where(predicate.Activity(sql.FieldNotNull(embeddingField)))
	}

	query = query.Order(func(s *sql.Selector) {
		var simExpr string
		if embeddingField != "" {
			vector := pgvector.NewVector(req.QueryEmbedding)
			simExpr = fmt.Sprintf("(1 - (%s <=> '%s'))", embeddingField, vector)
			if req.MinSimilarity > 0 {
				s.Where(sql.GT(simExpr, req.MinSimilarity))
			}
		} else {
			simExpr = "CAST(0 AS float8)"
		}
		s.AppendSelect(sql.As(simExpr, "similarity"))

		simWeight := req.SimilarityWeight
		socialWeight := req.SocialScoreWeight
		recencyWeight := req.RecencyWeight

		// Normalize weights if all are zero
		if simWeight == 0 && socialWeight == 0 && recencyWeight == 0 {
			simWeight = 1.0
			socialWeight = 0.0
			recencyWeight = 0.0
		}

		// Normalize weights to sum to 1
		totalWeight := simWeight + socialWeight + recencyWeight
		if totalWeight > 0 {
			simWeight = simWeight / totalWeight
			socialWeight = socialWeight / totalWeight
			recencyWeight = recencyWeight / totalWeight
		}

		// Some activities (e.g. rss feed items) don't have a social score,
		// so we fallback to a low popularity score for now,
		// to ensure they're not completely excluded from results.
		fallbackSocialScore := providers.NormSocialScore(20, 100)
		normalizedSocialScore := fmt.Sprintf("CASE WHEN social_score < 0 THEN %f ELSE social_score END", fallbackSocialScore)

		// Calculate time decay score (exponential decay over 30 days)
		// Score = e^(-k * days_old), where k controls decay rate
		// k = 0.1 means ~0.74 score after 3 days, ~0.37 after 10 days, ~0.05 after 30 days
		decayRate := 0.1
		recencyScoreExpr := fmt.Sprintf("EXP(-%f * EXTRACT(EPOCH FROM (NOW() - created_at)) / 86400)", decayRate)

		weightedExpr := fmt.Sprintf("((%s * %f) + (%s * %f) + (%s * %f))",
			simExpr, simWeight,
			normalizedSocialScore, socialWeight,
			recencyScoreExpr, recencyWeight)
		s.AppendSelect(sql.As(weightedExpr, "weighted_score"))
	})

	switch req.SortBy {
	case types.SortBySimilarity:
		if embeddingField != "" {
			query = query.Order(func(s *sql.Selector) {
				s.OrderExpr(sql.Expr("similarity DESC"))
			})
		} else {
			return nil, fmt.Errorf("sort by similarity requires query embedding parameter")
		}
	case types.SortByDate:
		query = query.Order(ent.Desc(entactivity.FieldCreatedAt))
	case types.SortBySocialScore:
		query = query.Order(func(s *sql.Selector) {
			s.OrderExpr(sql.Expr("social_score DESC"))
		})
	case types.SortByWeightedScore:
		query = query.Order(func(s *sql.Selector) {
			s.OrderExpr(sql.Expr("weighted_score DESC"))
		})
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
		entactivity.FieldSourceUids,
		entactivity.FieldSourceType,
		entactivity.FieldTitle,
		entactivity.FieldBody,
		entactivity.FieldURL,
		entactivity.FieldImageURL,
		entactivity.FieldCreatedAt,
		entactivity.FieldShortSummary,
		entactivity.FieldFullSummary,
		entactivity.FieldRawJSON,
		entactivity.FieldEmbedding1536,
		entactivity.FieldEmbedding3072,
		entactivity.FieldSocialScore,
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
		res, err := activityFromEnt(&a.Activity, float32(a.Similarity), len(req.QueryEmbedding), a.SourceUids)
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

func activityFromEnt(in *ent.Activity, similarity float32, embeddingLength int, sourceUIDs []string) (*types.DecoratedActivity, error) {
	act, err := activities.NewActivity(in.SourceType)
	if err != nil {
		return nil, fmt.Errorf("new activity: %w", err)
	}

	// TODO: Update how source foreign keys are tracked within the activity implementation.
	modifiedJSON, err := syncJSONSourceUIDs(in, sourceUIDs)
	if err != nil {
		return nil, fmt.Errorf("sync json source uids: %w", err)
	}

	err = act.UnmarshalJSON(modifiedJSON)
	if err != nil {
		return nil, fmt.Errorf("unmarshal activity: %w", err)
	}

	// Embeddings can be null if we clear them for reprocessing
	var embedding []float32
	switch embeddingLength {
	case 1536:
		embedding = in.Embedding1536.Slice()
	case 3072:
		embedding = in.Embedding3072.Slice()
	case 0:
		// Do nothing
	default:
		return nil, fmt.Errorf("invalid embedding length: %d", embeddingLength)
	}

	return &types.DecoratedActivity{
		Activity:   act,
		Embedding:  embedding,
		Similarity: similarity,
		Summary: &types.ActivitySummary{
			ShortSummary: in.ShortSummary,
			FullSummary:  in.FullSummary,
		},
	}, nil
}

func syncJSONSourceUIDs(in *ent.Activity, sourceUIDs []string) ([]byte, error) {
	// Hack: only top-level source_uids column is updated on write,
	// but source implementations depend on the source_uids field in the raw JSON.
	// For now just make sure the JSON field is in sync with the top-level column value.
	var rawActivity map[string]any
	if err := json.Unmarshal([]byte(in.RawJSON), &rawActivity); err != nil {
		return nil, fmt.Errorf("unmarshal raw json: %w", err)
	}
	rawActivity["source_ids"] = sourceUIDs

	modifiedJSON, err := json.Marshal(rawActivity)
	if err != nil {
		return nil, fmt.Errorf("marshal modified json: %w", err)
	}

	return modifiedJSON, nil
}
