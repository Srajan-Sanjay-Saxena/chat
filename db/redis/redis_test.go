package redis_test

import (
	"chat-v2/db/redis"
	"chat-v2/logger"
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMain(m *testing.M) {
	logger.TestInit()
	os.Exit(m.Run())
}

func TestPresenceStore_NilClient(t *testing.T) {
	// Test presence store behavior when Redis is unavailable or unconfigured (nil client)
	store := redis.NewPresenceStore(nil)
	ctx := context.Background()
	testUserID := uuid.New()

	// UpdatePresence on nil store should not panic and return nil
	err := store.UpdatePresence(ctx, testUserID)
	if err != nil {
		t.Fatalf("expected nil error on nil store UpdatePresence, got: %v", err)
	}

	// GetPresence on nil store should return Online: false
	presence, err := store.GetPresence(ctx, testUserID)
	if err != nil {
		t.Fatalf("expected nil error on nil store GetPresence, got: %v", err)
	}
	if presence.Online {
		t.Fatalf("expected presence.Online to be false on nil store, got true")
	}

	// GetMassPresence on nil store should return empty info
	info, err := store.GetMassPresence(ctx, []uuid.UUID{testUserID})
	if err != nil {
		t.Fatalf("expected nil error on nil store GetMassPresence, got: %v", err)
	}
	if len(info.OnlineMembers) != 0 {
		t.Fatalf("expected 0 online members on nil store, got: %d", len(info.OnlineMembers))
	}

	// Close on nil store should return nil
	if err := store.Close(); err != nil {
		t.Fatalf("expected nil error on nil store Close, got: %v", err)
	}
}

func TestConnect_EmptyAddress(t *testing.T) {
	client, err := redis.Connect("", "", "", 0, false)
	if err != nil {
		t.Fatalf("expected nil error for empty address, got: %v", err)
	}
	if client != nil {
		t.Fatalf("expected nil client for empty address, got: %v", client)
	}
}

func TestPresenceStore_Integration(t *testing.T) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	password := os.Getenv("REDIS_PASSWORD")
	username := os.Getenv("REDIS_USERNAME")

	client, err := redis.Connect(redisAddr, username, password, 0, false)
	if err != nil {
		t.Skipf("Skipping live Redis integration test (Redis server not reachable at %s): %v", redisAddr, err)
		return
	}
	defer client.Close()

	store := redis.NewPresenceStore(client)
	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()

	// Update presence for user1
	if err := store.UpdatePresence(ctx, user1); err != nil {
		t.Fatalf("failed to update presence for user1: %v", err)
	}

	// Check presence for user1 (should be online)
	p1, err := store.GetPresence(ctx, user1)
	if err != nil {
		t.Fatalf("failed to get presence for user1: %v", err)
	}
	if !p1.Online {
		t.Fatalf("expected user1 to be online, got offline")
	}
	if time.Since(p1.LastSeen) > 5*time.Second {
		t.Fatalf("unexpected last seen timestamp for user1: %v", p1.LastSeen)
	}

	// Check presence for user2 (not updated, should be offline)
	p2, err := store.GetPresence(ctx, user2)
	if err != nil {
		t.Fatalf("failed to get presence for user2: %v", err)
	}
	if p2.Online {
		t.Fatalf("expected user2 to be offline, got online")
	}

	// Mass presence check
	info, err := store.GetMassPresence(ctx, []uuid.UUID{user1, user2})
	if err != nil {
		t.Fatalf("failed mass presence check: %v", err)
	}
	if info.TotalMembers != 2 {
		t.Fatalf("expected total members 2, got %d", info.TotalMembers)
	}
	if len(info.OnlineMembers) != 1 || info.OnlineMembers[0] != user1 {
		t.Fatalf("expected online members [user1], got %#v", info.OnlineMembers)
	}
}
