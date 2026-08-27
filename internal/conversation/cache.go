package conversation

import (
	"chat-v2/internal/pkg/logger"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const participantCacheTTL = 1 * time.Hour

type ParticipantCache struct {
	redis *goredis.Client
	repo  *Repository
}

func NewParticipantCache(redis *goredis.Client, repo *Repository) *ParticipantCache {
	return &ParticipantCache{redis: redis, repo: repo}
}

func participantKey(convID uuid.UUID) string {
	return fmt.Sprintf("conv:%s:members", convID.String())
}

func (c *ParticipantCache) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	if c.redis == nil {
		return c.repo.IsParticipant(ctx, conversationID, userID)
	}

	key := participantKey(conversationID)

	exists, err := c.redis.Exists(ctx, key).Result()
	if err != nil {
		logger.Warn("Redis error, falling back to DB", "error", err)
		return c.repo.IsParticipant(ctx, conversationID, userID)
	}

	if exists > 0 {
		isMember, err := c.redis.SIsMember(ctx, key, userID.String()).Result()
		if err != nil {
			logger.Warn("Redis SISMEMBER error, falling back to DB", "error", err)
			return c.repo.IsParticipant(ctx, conversationID, userID)
		}
		return isMember, nil
	}

	// Cache miss
	members, err := c.repo.GetParticipants(ctx, conversationID)
	if err != nil {
		return false, err
	}

	go c.populate(conversationID, members)

	for _, memberID := range members {
		if memberID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (c *ParticipantCache) populate(conversationID uuid.UUID, members []uuid.UUID) {
	if c.redis == nil || len(members) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := participantKey(conversationID)

	pipe := c.redis.Pipeline()
	pipe.Del(ctx, key)

	memberStrs := make([]interface{}, len(members))
	for i, m := range members {
		memberStrs[i] = m.String()
	}
	pipe.SAdd(ctx, key, memberStrs...)
	pipe.Expire(ctx, key, participantCacheTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		logger.Warn("Failed to populate participant cache", "error", err)
	}
}

func (c *ParticipantCache) Add(ctx context.Context, conversationID, userID uuid.UUID) {
	if c.redis == nil {
		return
	}
	key := participantKey(conversationID)
	exists, _ := c.redis.Exists(ctx, key).Result()
	if exists > 0 {
		c.redis.SAdd(ctx, key, userID.String())
	}
}

func (c *ParticipantCache) Remove(ctx context.Context, conversationID, userID uuid.UUID) {
	if c.redis == nil {
		return
	}
	key := participantKey(conversationID)
	c.redis.SRem(ctx, key, userID.String())
}

func (c *ParticipantCache) Invalidate(ctx context.Context, conversationID uuid.UUID) {
	if c.redis == nil {
		return
	}
	c.redis.Del(ctx, participantKey(conversationID))
}
