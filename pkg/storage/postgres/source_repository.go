package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/defeedco/defeed/pkg/sources/types"

	"github.com/defeedco/defeed/pkg/sources"
	"github.com/defeedco/defeed/pkg/storage/postgres/ent"
	"github.com/defeedco/defeed/pkg/storage/postgres/ent/source"
)

type SourceRepository struct {
	db *DB
}

func NewSourceRepository(db *DB) *SourceRepository {
	return &SourceRepository{db: db}
}

func (r *SourceRepository) Add(s types.Source) error {
	ctx := context.Background()

	rawJson, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal source: %w", err)
	}

	_, err = r.db.Client().Source.Create().
		SetID(s.UID().String()).
		SetName(s.Name()).
		SetURL(s.URL()).
		SetType(s.UID().Type()).
		SetRawJSON(string(rawJson)).
		Save(ctx)

	return err
}

func (r *SourceRepository) Remove(uid string) error {
	ctx := context.Background()
	return r.db.Client().Source.DeleteOneID(uid).Exec(ctx)
}

func (r *SourceRepository) List() ([]types.Source, error) {
	ctx := context.Background()

	sourcesEnt, err := r.db.Client().Source.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]types.Source, len(sourcesEnt))
	for i, s := range sourcesEnt {
		out, err := sourceFromEnt(s)
		if err != nil {
			return nil, fmt.Errorf("deserialize source: %w", err)
		}
		result[i] = out
	}

	return result, nil
}

func (r *SourceRepository) GetByID(uid string) (types.Source, error) {
	ctx := context.Background()

	s, err := r.db.Client().Source.Query().Where(source.ID(uid)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return sourceFromEnt(s)
}

func sourceFromEnt(in *ent.Source) (types.Source, error) {
	out, err := sources.NewSource(in.Type)
	if err != nil {
		return nil, fmt.Errorf("new source: %w", err)
	}
	err = out.UnmarshalJSON([]byte(in.RawJSON))
	if err != nil {
		return nil, fmt.Errorf("unmarshal source: %w", err)
	}
	return out, nil
}
