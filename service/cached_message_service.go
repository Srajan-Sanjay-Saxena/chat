package service

import (
	"chat-v2/internal/cache"
	"chat-v2/logger"
	"chat-v2/repository"
	"context"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

type CachedMessageService struct {
	base         *MessageService
	repo         *repository.Repository
	cache        cache.Cache
	singleflight singleflight.Group
}

// NewCachedMessageService decorates MessageService with multi-level caching and singleflight protection.
func NewCachedMessageService(base *MessageService, repo *repository.Repository, cache cache.Cache) *CachedMessageService {
	return &CachedMessageService{
		base:  base,
		repo:  repo,
		cache: cache,
	}
}

// CreateMessage executes DB persistence, publishes via EventBus, and updates cache via Write-Through strategy.
func (s *CachedMessageService) CreateMessage(ctx context.Context, userID, conversationID uuid.UUID, content, username, clientID string) (*OutMessage, error) {
	outMsg, err := s.base.CreateMessage(ctx, userID, conversationID, content, username, clientID)
	if err != nil {
		return nil, err
	}

	// Update Cache (Write-Through)
	if s.cache != nil && outMsg != nil {
		cachedItem := toCachedMessage(outMsg)
		if cacheErr := s.cache.AddRecentMessage(ctx, conversationID, cachedItem); cacheErr != nil {
			if logger.Log != nil {
				logger.Log.Warn("Failed to append message to cache", "error", cacheErr, "conversation_id", conversationID)
			}
		}
	}

	return outMsg, nil
}

// GetRecentMessages retrieves messages for a conversation, checking cache first and using singleflight on cache miss.
func (s *CachedMessageService) GetRecentMessages(ctx context.Context, conversationID uuid.UUID) ([]*OutMessage, error) {
	// 1. Try Cache Read
	if s.cache != nil {
		cachedMsgs, err := s.cache.GetRecentMessages(ctx, conversationID)
		if err == nil && len(cachedMsgs) > 0 {
			if logger.Log != nil {
				logger.Log.Debug("Cache hit for conversation messages", "conversation_id", conversationID, "count", len(cachedMsgs))
			}
			outMsgs := make([]*OutMessage, 0, len(cachedMsgs))
			for _, cm := range cachedMsgs {
				outMsgs = append(outMsgs, fromCachedMessage(cm))
			}
			return outMsgs, nil
		}
	}

	// 2. Cache Miss: Coalesce concurrent DB queries using Singleflight
	sfKey := fmt.Sprintf("conv_msgs:%s", conversationID.String())
	v, err, _ := s.singleflight.Do(sfKey, func() (interface{}, error) {
		// Query DB for recent messages
		msgResp, repoErr := s.repo.GetMessagesByConversationID(ctx, conversationID, nil, 50)
		if repoErr != nil {
			return nil, repoErr
		}

		outMsgs := make([]*OutMessage, 0, len(msgResp.Messages))
		cachedMsgs := make([]*cache.CachedMessage, 0, len(msgResp.Messages))

		for _, dbMsg := range msgResp.Messages {
			outMsg := &OutMessage{
				Type:           "message",
				ID:             dbMsg.ID,
				SenderID:       dbMsg.SenderID,
				SenderUsername: dbMsg.SenderUsername,
				ConversationID: dbMsg.ConversationID,
				Content:        dbMsg.Content,
				CreatedAt:      dbMsg.CreatedAt,
			}
			outMsgs = append(outMsgs, outMsg)
			cachedMsgs = append(cachedMsgs, toCachedMessage(outMsg))
		}

		// Populate cache asynchronously
		if s.cache != nil && len(cachedMsgs) > 0 {
			go func() {
				_ = s.cache.SetRecentMessages(context.Background(), conversationID, cachedMsgs)
			}()
		}

		return outMsgs, nil
	})

	if err != nil {
		return nil, err
	}

	return v.([]*OutMessage), nil
}

func toCachedMessage(m *OutMessage) *cache.CachedMessage {
	if m == nil {
		return nil
	}
	return &cache.CachedMessage{
		Type:           m.Type,
		ID:             m.ID,
		SenderID:       m.SenderID,
		SenderUsername: m.SenderUsername,
		ConversationID: m.ConversationID,
		Content:        m.Content,
		CreatedAt:      m.CreatedAt,
	}
}

func fromCachedMessage(m *cache.CachedMessage) *OutMessage {
	if m == nil {
		return nil
	}
	return &OutMessage{
		Type:           m.Type,
		ID:             m.ID,
		SenderID:       m.SenderID,
		SenderUsername: m.SenderUsername,
		ConversationID: m.ConversationID,
		Content:        m.Content,
		CreatedAt:      m.CreatedAt,
	}
}
