package realtime

import (
	"chat-v2/service"
	"context"
	"github.com/google/uuid"
	// "chat-v2/db"
	"time"
)

type outMessage struct {
	Type           string    `json:"type"`
	ID             uuid.UUID    `json:"id"`
	SenderID       uuid.UUID    `json:"sender_id"`
	ConversationID uuid.UUID    `json:"conversation_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time   `json:"created_at"`
	SenderUsername string    `json:"sender_username"`
}

type LocalBus struct {
	handler service.MessageHandler
}

func NewLocalBus(localHandler service.MessageHandler) *LocalBus {
	return &LocalBus{handler: localHandler}
}

func (b *LocalBus) Publish(ctx context.Context, outMsg *service.OutMessage) error {
	b.handler(outMsg)
	return nil;
}

func (b *LocalBus) Subscribe(ctx context.Context, handler service.MessageHandler) error {
	b.handler = handler
	return nil
}

