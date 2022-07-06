package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Endpoint holds the schema definition for the Endpoint entity.
type Endpoint struct {
	ent.Schema
}

// Fields of the Endpoint.
func (Endpoint) Fields() []ent.Field {
	return []ent.Field{
		field.String("url").
			NotEmpty(),
	}
}

// Edges of the Endpoint.
func (Endpoint) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("links", Endpoint.Type),
	}
}
