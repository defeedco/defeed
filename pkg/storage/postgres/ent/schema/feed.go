package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/glanceapp/glance/pkg/feeds"
)

type Feed struct {
	ent.Schema
}

func (Feed) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").Unique(),
		field.String("user_id"),
		field.String("name"),
		field.String("icon"),
		field.String("query"),
		field.JSON("source_uids", []string{}),
		field.Time("created_at"),
		field.Time("updated_at"),
		field.JSON("summaries", []feeds.FeedSummary{}).Optional(),
	}
}

func (Feed) Edges() []ent.Edge {
	return nil
}
