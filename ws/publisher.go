package ws

import (
	// "chat-v2/db"
	"chat-v2/logger"
	"chat-v2/service"
	"context"
	"encoding/json"
)

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
	case p.hub.broadcast <- broadcastMessage{message: messageBytes, conversationID: msg.ConversationID}:
		return nil
	}
}
