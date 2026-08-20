package service

import (
	"chat-v2/logger"
	"chat-v2/repository"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	participantCacheTTL = 1 * time.Hour
)

// ParticipantCache provides a Redis-backed cache for conversation membership checks.
// Uses a Redis SET per conversation storing member user IDs.
type ParticipantCache struct {
	redis *redis.Client
	repo  *repository.Repository
}

// NewParticipantCache creates a new ParticipantCache instance.
// If redis is nil, all operations fall through to the database.
func NewParticipantCache(redisClient *redis.Client, repo *repository.Repository) *ParticipantCache {
	return &ParticipantCache{
		redis: redisClient,
		repo:  repo,
	}
}

func participantKey(convID uuid.UUID) string {
	return fmt.Sprintf("conv:%s:members", convID.String())
}

// IsParticipant checks if a user is a member of a conversation.
// First checks Redis SET, falls back to DB on cache miss and populates cache.
func (pc *ParticipantCache) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	if pc.redis == nil {
		return pc.repo.IsParticipant(ctx, conversationID, userID)
	}

	key := participantKey(conversationID)

	// Check if the SET exists in Redis (cache hit vs miss)
	exists, err := pc.redis.Exists(ctx, key).Result()
	if err != nil {
		// Redis error — fall through to DB
		logger.Log.Warn("Redis error on participant cache check, falling back to DB", "error", err)
		return pc.repo.IsParticipant(ctx, conversationID, userID)
	}

	if exists > 0 {
		// Cache hit — check membership with SISMEMBER
		isMember, err := pc.redis.SIsMember(ctx, key, userID.String()).Result()
		if err != nil {
			logger.Log.Warn("Redis SISMEMBER error, falling back to DB", "error", err)
			return pc.repo.IsParticipant(ctx, conversationID, userID)
		}
		return isMember, nil
	}

	// Cache miss — load members from DB and populate cache
	members, err := pc.repo.GetParticipantsByConversationID(ctx, conversationID)
	if err != nil {
		return false, err
	}

	// Populate cache asynchronously
	go pc.populateCache(conversationID, members)

	// Check membership in the loaded list
	for _, memberID := range members {
		if memberID == userID {
			return true, nil
		}
	}
	return false, nil
}

// populateCache fills the Redis SET with member IDs for a conversation.
func (pc *ParticipantCache) populateCache(conversationID uuid.UUID, members []uuid.UUID) {
	if pc.redis == nil || len(members) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := participantKey(conversationID)

	// Use pipeline: DEL + SADD + EXPIRE
	pipe := pc.redis.Pipeline()
	pipe.Del(ctx, key)

	memberStrs := make([]interface{}, len(members))
	for i, m := range members {
		memberStrs[i] = m.String()
	}
	pipe.SAdd(ctx, key, memberStrs...)
	pipe.Expire(ctx, key, participantCacheTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		logger.Log.Warn("Failed to populate participant cache", "error", err, "conversation_id", conversationID)
	}
}

// AddParticipant adds a user to the cached member set (call after DB write succeeds).
func (pc *ParticipantCache) AddParticipant(ctx context.Context, conversationID, userID uuid.UUID) {
	if pc.redis == nil {
		return
	}
	key := participantKey(conversationID)
	// Only add if the set exists (don't create a stale partial set)
	exists, err := pc.redis.Exists(ctx, key).Result()
	if err != nil || exists == 0 {
		return
	}
	if err := pc.redis.SAdd(ctx, key, userID.String()).Err(); err != nil {
		logger.Log.Warn("Failed to add participant to cache", "error", err, "conversation_id", conversationID, "user_id", userID)
	}
}

// RemoveParticipant removes a user from the cached member set (call after DB write succeeds).
func (pc *ParticipantCache) RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) {
	if pc.redis == nil {
		return
	}
	key := participantKey(conversationID)
	if err := pc.redis.SRem(ctx, key, userID.String()).Err(); err != nil {
		logger.Log.Warn("Failed to remove participant from cache", "error", err, "conversation_id", conversationID, "user_id", userID)
	}
}

// InvalidateConversation removes the entire cached member set for a conversation.
func (pc *ParticipantCache) InvalidateConversation(ctx context.Context, conversationID uuid.UUID) {
	if pc.redis == nil {
		return
	}
	key := participantKey(conversationID)
	if err := pc.redis.Del(ctx, key).Err(); err != nil {
		logger.Log.Warn("Failed to invalidate participant cache", "error", err, "conversation_id", conversationID)
	}
}
