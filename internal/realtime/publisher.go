package realtime

import (
	"chat-v2/logger"
	"chat-v2/service"
	"context"
	"encoding/json"
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

type LocalPublisher struct {
	hub *Hub
}

func NewLocalPublisher(hub *Hub) service.EventPublisher {
	return &LocalPublisher{hub: hub}
}

func (p *LocalPublisher) PublishMessage(ctx context.Context, msg *service.OutMessage) error {
	if p == nil || p.hub == nil || msg == nil {
		return nil
	}

	outMsg := outMessage{
		Type:           "message",
		ID:             msg.ID,
		SenderID:       msg.SenderID,
		SenderUsername: msg.SenderUsername,
		ConversationID: msg.ConversationID,
		Content:        msg.Content,
		CreatedAt:      msg.CreatedAt,
	}

	messageBytes, err := json.Marshal(outMsg)
	if err != nil {
		logger.Log.Error("error marshalling outgoing message", "error", err)
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.hub.broadcast <- broadcastRequest{message: messageBytes, conversationID: msg.ConversationID}:
		logger.Log.Debug("message published to hub broadcast channel", "message_id", msg.ID, "conversation_id", msg.ConversationID)
		return nil
	}
}
