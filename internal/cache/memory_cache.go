package cache

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type memoryItem struct {
	messages []*CachedMessage
	expires  time.Time
}

type MemoryCache struct {
	items map[uuid.UUID]*memoryItem
	mu    sync.RWMutex
	ttl   time.Duration
}

// NewMemoryCache initializes an in-memory cache fallback.
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &MemoryCache{
		items: make(map[uuid.UUID]*memoryItem),
		ttl:   ttl,
	}
}

func (m *MemoryCache) GetRecentMessages(ctx context.Context, conversationID uuid.UUID) ([]*CachedMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, found := m.items[conversationID]
	if !found || time.Now().After(item.expires) {
		return nil, nil // Cache miss
	}

	return item.messages, nil
}

func (m *MemoryCache) SetRecentMessages(ctx context.Context, conversationID uuid.UUID, messages []*CachedMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[conversationID] = &memoryItem{
		messages: messages,
		expires:  time.Now().Add(m.ttl),
	}
	return nil
}

func (m *MemoryCache) AddRecentMessage(ctx context.Context, conversationID uuid.UUID, msg *CachedMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, found := m.items[conversationID]
	if !found || time.Now().After(item.expires) {
		m.items[conversationID] = &memoryItem{
			messages: []*CachedMessage{msg},
			expires:  time.Now().Add(m.ttl),
		}
		return nil
	}

	item.messages = append(item.messages, msg)
	if len(item.messages) > 100 {
		item.messages = item.messages[len(item.messages)-100:]
	}
	item.expires = time.Now().Add(m.ttl)
	return nil
}

func (m *MemoryCache) InvalidateConversation(ctx context.Context, conversationID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, conversationID)
	return nil
}
