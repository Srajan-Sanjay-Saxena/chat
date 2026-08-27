package conversation

import (
	"chat-v2/internal/pkg/logger"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const participantCacheTTL = 1 * time.Hour

type ParticipantCache struct {
	redis      *goredis.Client
	repo       *Repository
	populating sync.Map // conversationID -> struct{}, prevents concurrent populate storms
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

	// Try cache first
	exists, err := c.redis.Exists(ctx, key).Result()
	if err == nil && exists > 0 {
		isMember, err := c.redis.SIsMember(ctx, key, userID.String()).Result()
		if err == nil {
			return isMember, nil
		}
	}

	// Cache miss or Redis error — ask DB directly (targeted EXISTS query, cheap)
	isMember, err := c.repo.IsParticipant(ctx, conversationID, userID)
	if err != nil {
		return false, err
	}

	// Populate cache in background (deduped — only one populate per conversation at a time)
	go c.populateIfMissing(conversationID)

	return isMember, nil
}

func (c *ParticipantCache) populateIfMissing(conversationID uuid.UUID) {
	
	if c.redis == nil {
		return
	}

	// Deduplicate concurrent populate attempts for the same conversation
	key := conversationID.String()
	if _, loaded := c.populating.LoadOrStore(key, struct{}{}); loaded {
		return // another goroutine is already populating this conversation
	}
	defer c.populating.Delete(key)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	redisKey := participantKey(conversationID)

	// Check again — another request may have populated while we were queued
	exists, _ := c.redis.Exists(ctx, redisKey).Result()
	if exists > 0 {
		return
	}

	members, err := c.repo.GetParticipants(ctx, conversationID)
	if err != nil || len(members) == 0 {
		return
	}

	c.populate(conversationID, members)
	
}

func (c *ParticipantCache) populate(conversationID uuid.UUID, members []uuid.UUID) {
	
	if c.redis == nil || len(members) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := participantKey(conversationID)

	// Use a pipeline to set the members and expiration atomically
	pipe := c.redis.Pipeline()
	pipe.Del(ctx, key) // Clear any existing members before adding new ones

	memberStrs := make([]any, len(members))
	for i, m := range members {
		memberStrs[i] = m.String()
	}

	pipe.SAdd(ctx, key, memberStrs...) // Add all members to the set
	pipe.Expire(ctx, key, participantCacheTTL) // Set expiration for the cache

	// Execute the pipeline

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
