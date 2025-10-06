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
		field.JSON("source_uids", []string{}),
		field.String("source_type"),
		field.String("title"),
		field.String("body"),
		field.String("url"),
		field.String("image_url"),
		field.Time("created_at"),
		field.String("short_summary"),
		field.String("full_summary"),
		field.String("raw_json"),
		field.Other("embedding_1536", pgvector.Vector{}).
			SchemaType(map[string]string{
				dialect.Postgres: "vector(1536)",
			}).
			Nillable().
			Optional(),
		field.Other("embedding_3072", pgvector.Vector{}).
			SchemaType(map[string]string{
				dialect.Postgres: "vector(3072)",
			}).
			Nillable().
			Optional(),
		field.Float("social_score").
			Default(-1.0),
		// Internal field for monitoring purposes
		field.Int("update_count").
			Default(0),
	}
}

func (Activity) Edges() []ent.Edge {
	return nil
}
