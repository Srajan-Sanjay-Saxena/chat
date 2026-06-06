package service

import (
	"chat-v2/db"
	"chat-v2/repository"
	"context"
	"errors"
	"strings"
	"time"
	"chat-v2/logger"
	"github.com/google/uuid"
)

var ErrNotParticipant = errors.New("not participant")
var ErrInvalidMessage = errors.New("invalid message")

type MessageService struct {
	repo          *repository.Repository
	isParticipant func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	publisher     EventPublisher
}

func NewMessageService(repo *repository.Repository, publisher EventPublisher, isParticipant func(context.Context, uuid.UUID, uuid.UUID) (bool, error)) *MessageService {
	return &MessageService{repo: repo, isParticipant: isParticipant, publisher: publisher}
}

func (s *MessageService) CreateMessage(ctx context.Context, userID, conversationID uuid.UUID, content string) (*db.Message, error) {
	start := time.Now()

	if s == nil || s.repo == nil {
		return nil, errors.New("message service is not initialized")
	}
	if userID == uuid.Nil || conversationID == uuid.Nil || strings.TrimSpace(content) == "" {
		return nil, ErrInvalidMessage
	}

	checkStart := time.Now()
	if s.isParticipant != nil {
		allowed, err := s.isParticipant(ctx, conversationID, userID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, ErrNotParticipant
		}
	}

	logger.Log.Debug("participant check completed", "duration_ms", time.Since(checkStart).Milliseconds())

	message := &db.Message{
		ID:             uuid.New(),
		SenderID:       userID,
		ConversationID: conversationID,
		Content:        content,
		CreatedAt:      time.Now().UTC(),
	}

	insertStart := time.Now()
	if err := s.repo.CreateMessage(ctx, message); err != nil {
		return nil, err
	}
	logger.Log.Debug("message inserted into database", "message_id", message.ID, "duration_ms", time.Since(insertStart).Milliseconds())
	// Publish the message to subscribers
	if s.publisher == nil {
		return message, nil
	}
	publishStart := time.Now()
	if err := s.publisher.PublishMessage(ctx, message); err != nil {
		return nil, err
	}
	logger.Log.Debug("message published to subscribers", "message_id", message.ID, "duration_ms", time.Since(publishStart).Milliseconds())
	logger.Log.Debug("CreateMessage completed", "message_id", message.ID, "total_duration_ms", time.Since(start).Milliseconds())
	return message, nil
}
