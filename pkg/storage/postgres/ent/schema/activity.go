package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"github.com/pgvector/pgvector-go"
)

type Activity struct {
	ent.Schema
}

func (Activity) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").Unique(),
		field.String("uid").Unique(),
		field.String("source_uid"),
		field.String("source_type"),
		field.String("title"),
		field.String("body"),
		field.String("url"),
		field.String("image_url"),
		field.Time("created_at"),
		field.String("short_summary"),
		field.String("full_summary"),
		field.String("raw_json"),
		field.Other("embedding", pgvector.Vector{}).
			SchemaType(map[string]string{
				// Use text-embedding-3-large output dimensions
				dialect.Postgres: "vector(3072)",
			}).
			Nillable().
			Optional(),
	}
}

func (Activity) Edges() []ent.Edge {
	return nil
}
