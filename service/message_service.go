package service

import (
	"chat-v2/db"
	"chat-v2/logger"
	"chat-v2/repository"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrNotParticipant = errors.New("not participant")
var ErrInvalidMessage = errors.New("invalid message")
var ErrDuplicateMessage = errors.New("duplicate message")

const dedupTTL = 5 * time.Minute

type MessageProcessor interface {
	CreateMessage(ctx context.Context, userID, conversationID uuid.UUID, content, username, clientID string) (*OutMessage, error)
}

type MessageService struct {
	repo             *repository.Repository
	publisher        EventBus
	participantCache *ParticipantCache
	redis            *redis.Client
}

func NewMessageService(repo *repository.Repository, publisher EventBus, participantCache *ParticipantCache, redisClient *redis.Client) *MessageService {
	return &MessageService{
		repo:             repo,
		publisher:        publisher,
		participantCache: participantCache,
		redis:            redisClient,
	}
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

func (s *MessageService) CreateMessage(ctx context.Context, userID, conversationID uuid.UUID, content, username, clientID string) (*OutMessage, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("message service is not initialized")
	}
	if userID == uuid.Nil || conversationID == uuid.Nil || strings.TrimSpace(content) == "" {
		return nil, ErrInvalidMessage
	}

	// Idempotency check: if client provided a clientID, check dedup key
	if clientID != "" && s.redis != nil {
		dedupKey := fmt.Sprintf("dedup:msg:%s", clientID)
		// SETNX: only succeeds if key doesn't exist
		set, err := s.redis.SetNX(ctx, dedupKey, "1", dedupTTL).Result()
		if err != nil {
			logger.Log.Warn("Redis dedup check failed, proceeding without dedup", "error", err, "client_id", clientID)
		} else if !set {
			// Key already existed — this is a duplicate
			logger.Log.Debug("Duplicate message detected, skipping", "client_id", clientID)
			return nil, ErrDuplicateMessage
		}
	}

	// Participant check: use cached version if available
	var isParticipant bool
	var err error
	if s.participantCache != nil {
		isParticipant, err = s.participantCache.IsParticipant(ctx, conversationID, userID)
	} else {
		isParticipant, err = s.repo.IsParticipant(ctx, conversationID, userID)
	}
	if err != nil {
		return nil, err
	}
	if !isParticipant {
		return nil, ErrNotParticipant
	}

	message := &db.Message{
		ID:             uuid.New(),
		SenderID:       userID,
		ConversationID: conversationID,
		Content:        content,
		CreatedAt:      time.Now().UTC(),
	}

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

	// Publish the message to subscribers
	if s.publisher == nil {
		return outMsg, nil
	}
	if err := s.publisher.Publish(ctx, outMsg); err != nil {
		return nil, err
	}

	return outMsg, nil
}
