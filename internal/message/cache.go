package message

import (
	"chat-v2/internal/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

type Cache interface {
	GetRecent(ctx context.Context, conversationID uuid.UUID) ([]*Message, error)
	SetRecent(ctx context.Context, conversationID uuid.UUID, messages []*Message) error
	AddMessage(ctx context.Context, conversationID uuid.UUID, msg *Message) error
	Invalidate(ctx context.Context, conversationID uuid.UUID) error
}

func cacheKey(convID uuid.UUID) string {
	return fmt.Sprintf("cache:conv:%s:messages", convID.String())
}

// RedisCache implements Cache using Redis ZSET
type RedisCache struct {
	client *goredis.Client
	ttl    time.Duration
}

func NewRedisCache(client *goredis.Client, ttl time.Duration) *RedisCache {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &RedisCache{client: client, ttl: ttl}
}

func (c *RedisCache) GetRecent(ctx context.Context, conversationID uuid.UUID) ([]*Message, error) {
	if c.client == nil {
		return nil, nil
	}

	key := cacheKey(conversationID)
	vals, err := c.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil || len(vals) == 0 {
		return nil, err
	}

	messages := make([]*Message, 0, len(vals))
	for _, val := range vals {
		var msg Message
		if err := json.Unmarshal([]byte(val), &msg); err != nil {
			continue
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

func (c *RedisCache) SetRecent(ctx context.Context, conversationID uuid.UUID, messages []*Message) error {
	if c.client == nil || len(messages) == 0 {
		return nil
	}

	key := cacheKey(conversationID)
	pipe := c.client.Pipeline()
	pipe.Del(ctx, key)

	for _, msg := range messages {
		payload, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		score := float64(msg.CreatedAt.UnixNano() / int64(time.Millisecond))
		pipe.ZAdd(ctx, key, goredis.Z{Score: score, Member: string(payload)})
	}

	pipe.ZRemRangeByRank(ctx, key, 0, -101)
	pipe.Expire(ctx, key, c.ttl)

	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) AddMessage(ctx context.Context, conversationID uuid.UUID, msg *Message) error {
	if c.client == nil || msg == nil {
		return nil
	}

	key := cacheKey(conversationID)
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	score := float64(msg.CreatedAt.UnixNano() / int64(time.Millisecond))
	pipe := c.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: score, Member: string(payload)})
	pipe.ZRemRangeByRank(ctx, key, 0, -101)
	pipe.Expire(ctx, key, c.ttl)

	_, err = pipe.Exec(ctx)
	return err
}

func (c *RedisCache) Invalidate(ctx context.Context, conversationID uuid.UUID) error {
	if c.client == nil {
		return nil
	}
	return c.client.Del(ctx, cacheKey(conversationID)).Err()
}

// MemoryCache implements Cache using in-memory map
type MemoryCache struct {
	items map[uuid.UUID]*memoryItem
	mu    sync.RWMutex
	ttl   time.Duration
}

type memoryItem struct {
	messages []*Message
	expires  time.Time
}

func NewMemoryCache(ttl time.Duration) *MemoryCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &MemoryCache{
		items: make(map[uuid.UUID]*memoryItem),
		ttl:   ttl,
	}
}

func (c *MemoryCache) GetRecent(ctx context.Context, conversationID uuid.UUID) ([]*Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[conversationID]
	if !found || time.Now().After(item.expires) {
		return nil, nil
	}

	return item.messages, nil
}

func (c *MemoryCache) SetRecent(ctx context.Context, conversationID uuid.UUID, messages []*Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[conversationID] = &memoryItem{
		messages: messages,
		expires:  time.Now().Add(c.ttl),
	}
	return nil
}

func (c *MemoryCache) AddMessage(ctx context.Context, conversationID uuid.UUID, msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, found := c.items[conversationID]
	if !found || time.Now().After(item.expires) {
		c.items[conversationID] = &memoryItem{
			messages: []*Message{msg},
			expires:  time.Now().Add(c.ttl),
		}
		return nil
	}

	item.messages = append(item.messages, msg)
	if len(item.messages) > 100 {
		item.messages = item.messages[len(item.messages)-100:]
	}
	item.expires = time.Now().Add(c.ttl)
	return nil
}

func (c *MemoryCache) Invalidate(ctx context.Context, conversationID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, conversationID)
	return nil
}

// CachedService wraps Service with caching
type CachedService struct {
	*Service
	cache Cache
}

func NewCachedService(svc *Service, cache Cache) *CachedService {
	return &CachedService{Service: svc, cache: cache}
}

func (s *CachedService) Create(ctx context.Context, userID, conversationID uuid.UUID, content, username, clientID string) (*OutMessage, error) {
	outMsg, err := s.Service.Create(ctx, userID, conversationID, content, username, clientID)
	if err != nil {
		return nil, err
	}

	if s.cache != nil && outMsg != nil {
		cacheMsg := &Message{
			ID:             outMsg.ID,
			SenderID:       outMsg.SenderID,
			SenderUsername: outMsg.SenderUsername,
			ConversationID: outMsg.ConversationID,
			Content:        outMsg.Content,
			CreatedAt:      outMsg.CreatedAt,
		}
		if err := s.cache.AddMessage(ctx, conversationID, cacheMsg); err != nil {
			logger.Warn("Failed to cache message", "error", err)
		}
	}

	return outMsg, nil
}
