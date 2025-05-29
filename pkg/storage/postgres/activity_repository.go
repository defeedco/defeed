package postgres

import (
	"context"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/activities"
	"github.com/glanceapp/glance/pkg/sources/activities/types"

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
