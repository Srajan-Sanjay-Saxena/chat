package service

import (
	"chat-v2/db"
	"chat-v2/repository"
	"context"
	"errors"
	"strings"
	"time"
	// "chat-v2/logger"
	"github.com/google/uuid"
)

var ErrNotParticipant = errors.New("not participant")
var ErrInvalidMessage = errors.New("invalid message")

type MessageService struct {
	repo          *repository.Repository
	publisher     EventPublisher
}

func NewMessageService(repo *repository.Repository, publisher EventPublisher) *MessageService {
	return &MessageService{repo: repo, publisher: publisher}
}

type OutMessage struct {
	Type           string    `json:"type"`
	ID             uuid.UUID `json:"id"`
	SenderID       uuid.UUID `json:"sender_id"`
	SenderUsername string    `json:"sender_username"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *MessageService) CreateMessage(ctx context.Context, userID, conversationID uuid.UUID, content,username string) (*OutMessage, error) {
	// start := time.Now()

	if s == nil || s.repo == nil {
		return nil, errors.New("message service is not initialized")
	}
	if userID == uuid.Nil || conversationID == uuid.Nil || strings.TrimSpace(content) == "" {
		return nil, ErrInvalidMessage
	}

	// checkStart := time.Now()
	isParticipant, err := s.repo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isParticipant {
		return nil, ErrNotParticipant
	}

	// logger.Log.Debug("participant check completed", "duration_ms", time.Since(checkStart).Milliseconds())

	message := &db.Message{
		ID:             uuid.New(),
		SenderID:       userID,
		ConversationID: conversationID,
		Content:        content,
		CreatedAt:      time.Now().UTC(),
	}

	// insertStart := time.Now()
	if err := s.repo.CreateMessage(ctx, message); err != nil {
		return nil, err
	}

	outMsg := &OutMessage{
		Type:           "message",
		ID:             message.ID,
		SenderID:       message.SenderID,
		SenderUsername: username,
		ConversationID: message.ConversationID,
		Content:        message.Content,
		CreatedAt:      message.CreatedAt,
	}

	// logger.Log.Debug("message inserted into database", "message_id", message.ID, "duration_ms", time.Since(insertStart).Milliseconds())
	// Publish the message to subscribers
	if s.publisher == nil {
		return outMsg, nil
	}
	// publishStart := time.Now()
	if err := s.publisher.PublishMessage(ctx, outMsg); err != nil {
		return nil, err
	}
	// logger.Log.Debug("message published to subscribers", "message_id", message.ID, "duration_ms", time.Since(publishStart).Milliseconds())
	// logger.Log.Debug("CreateMessage completed", "message_id", message.ID, "total_duration_ms", time.Since(start).Milliseconds())


	
	return outMsg, nil
}
