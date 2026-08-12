package cache

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CachedMessage defines the serializable struct stored in cache.
type CachedMessage struct {
	Type           string    `json:"type"`
	ID             uuid.UUID `json:"id"`
	SenderID       uuid.UUID `json:"sender_id"`
	SenderUsername string    `json:"sender_username"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// Cache interface defines operations for caching recent messages and metadata.
type Cache interface {
	GetRecentMessages(ctx context.Context, conversationID uuid.UUID) ([]*CachedMessage, error)
	SetRecentMessages(ctx context.Context, conversationID uuid.UUID, messages []*CachedMessage) error
	AddRecentMessage(ctx context.Context, conversationID uuid.UUID, msg *CachedMessage) error
	InvalidateConversation(ctx context.Context, conversationID uuid.UUID) error
}

// Key format helpers
func ConvMessagesKey(convID uuid.UUID) string {
	return "cache:conv:" + convID.String() + ":messages"
}

func UserProfileKey(userID uuid.UUID) string {
	return "cache:user:" + userID.String()
}

func ConvMembersKey(convID uuid.UUID) string {
	return "cache:conv:" + convID.String() + ":members"
}
