package cache

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMemoryCache_Operations(t *testing.T) {
	ctx := context.Background()
	memCache := NewMemoryCache(5 * time.Minute)
	convID := uuid.New()

	// 1. Initial Cache Miss
	msgs, err := memCache.GetRecentMessages(ctx, convID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected empty slice on cache miss, got %d", len(msgs))
	}

	// 2. Add Message
	msg1 := &CachedMessage{
		ID:             uuid.New(),
		ConversationID: convID,
		Content:        "Hello World",
		CreatedAt:      time.Now(),
	}

	if err := memCache.AddRecentMessage(ctx, convID, msg1); err != nil {
		t.Fatalf("unexpected error on add: %v", err)
	}

	// 3. Cache Hit
	msgs, err = memCache.GetRecentMessages(ctx, convID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "Hello World" {
		t.Fatalf("expected 1 message 'Hello World', got %+v", msgs)
	}

	// 4. Invalidate
	if err := memCache.InvalidateConversation(ctx, convID); err != nil {
		t.Fatalf("unexpected error on invalidate: %v", err)
	}

	msgs, err = memCache.GetRecentMessages(ctx, convID)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after invalidation, got %d", len(msgs))
	}
}
