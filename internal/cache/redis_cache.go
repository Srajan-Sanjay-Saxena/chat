package cache

import (
	"chat-v2/logger"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisCache returns a new RedisCache instance. Default TTL is 24 hours.
func NewRedisCache(client *redis.Client, ttl time.Duration) *RedisCache {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &RedisCache{
		client: client,
		ttl:    ttl,
	}
}

// GetRecentMessages retrieves recent messages from Redis ZSET.
func (c *RedisCache) GetRecentMessages(ctx context.Context, conversationID uuid.UUID) ([]*CachedMessage, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("redis client is nil")
	}

	key := ConvMessagesKey(conversationID)
	// Fetch messages ordered by timestamp ascending
	vals, err := c.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	if len(vals) == 0 {
		return nil, nil // Cache miss
	}

	messages := make([]*CachedMessage, 0, len(vals))
	for _, val := range vals {
		var msg CachedMessage
		if err := json.Unmarshal([]byte(val), &msg); err != nil {
			if logger.Log != nil {
				logger.Log.Warn("Failed to unmarshal cached message", "error", err, "key", key)
			}
			continue
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

// SetRecentMessages sets recent messages in Redis ZSET.
func (c *RedisCache) SetRecentMessages(ctx context.Context, conversationID uuid.UUID, messages []*CachedMessage) error {
	if c == nil || c.client == nil || len(messages) == 0 {
		return nil
	}

	key := ConvMessagesKey(conversationID)
	pipe := c.client.Pipeline()
	pipe.Del(ctx, key)

	for _, msg := range messages {
		payload, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		score := float64(msg.CreatedAt.UnixNano() / int64(time.Millisecond))
		pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: string(payload)})
	}

	// Keep top 100 messages
	pipe.ZRemRangeByRank(ctx, key, 0, -101)
	pipe.Expire(ctx, key, c.ttl)

	_, err := pipe.Exec(ctx)
	return err
}

// AddRecentMessage appends a single message to Redis ZSET and trims size to 100.
func (c *RedisCache) AddRecentMessage(ctx context.Context, conversationID uuid.UUID, msg *CachedMessage) error {
	if c == nil || c.client == nil || msg == nil {
		return nil
	}

	key := ConvMessagesKey(conversationID)
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	score := float64(msg.CreatedAt.UnixNano() / int64(time.Millisecond))
	pipe := c.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: string(payload)})
	pipe.ZRemRangeByRank(ctx, key, 0, -101)
	pipe.Expire(ctx, key, c.ttl)

	_, err = pipe.Exec(ctx)
	return err
}

// InvalidateConversation deletes the cached messages for a conversation.
func (c *RedisCache) InvalidateConversation(ctx context.Context, conversationID uuid.UUID) error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Del(ctx, ConvMessagesKey(conversationID)).Err()
}
