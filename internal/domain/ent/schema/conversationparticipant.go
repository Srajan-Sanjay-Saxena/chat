package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ConversationParticipant holds the schema definition for the ConversationParticipant entity.
type ConversationParticipant struct {
	ent.Schema
}

// Fields of the ConversationParticipant.
func (ConversationParticipant) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.UUID("conversation_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the ConversationParticipant.
func (ConversationParticipant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("conversation", Conversation.Type).
			Ref("participants").
			Field("conversation_id").
			Unique().
			Required(),
		edge.From("user", User.Type).
			Ref("participations").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes of the ConversationParticipant.
func (ConversationParticipant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("conversation_id", "user_id").Unique(),
		index.Fields("user_id"),
	}
}
