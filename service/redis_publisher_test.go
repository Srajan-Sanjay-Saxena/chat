package service_test

import (
	"chat-v2/db/redis"
	"chat-v2/logger"
	"chat-v2/service"
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

func TestRedisBus_NilClient(t *testing.T) {
	bus := service.NewRedisBus(nil)
	ctx := context.Background()

	outMsg := &service.OutMessage{
		Type:           "message",
		ID:             uuid.New(),
		SenderID:       uuid.New(),
		SenderUsername: "testuser",
		ConversationID: uuid.New(),
		Content:        "Hello nil test",
		CreatedAt:      time.Now(),
	}

	// Publish with nil RedisClient should not panic
	err := bus.Publish(ctx, outMsg)
	if err == nil {
		// Expecting error or silent return, but definitely NO panic
		t.Log("Publish on nil RedisBus returned nil error without panicking")
	}

	// Subscribe with nil RedisClient should not panic
	err = bus.Subscribe(ctx, func(m *service.OutMessage) {})
	if err == nil {
		t.Log("Subscribe on nil RedisBus returned nil error without panicking")
	}
}

func TestRedisBus_PubSub_Delivery(t *testing.T) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	password := os.Getenv("REDIS_PASSWORD")
	username := os.Getenv("REDIS_USERNAME")

	client, err := redis.Connect(redisAddr, username, password, 0, false)
	if err != nil || client == nil {
		t.Skipf("Skipping live Redis PubSub test (Redis server not available at %s): %v", redisAddr, err)
		return
	}
	defer client.Close()

	bus := service.NewRedisBus(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	convID := uuid.New()
	msgID := uuid.New()
	testContent := "Real-time PubSub Test Message " + msgID.String()

	receivedChan := make(chan *service.OutMessage, 1)

	// Subscribe to pubsub messages
	err = bus.Subscribe(ctx, func(msg *service.OutMessage) {
		if msg.ID == msgID {
			receivedChan <- msg
		}
	})
	if err != nil {
		t.Fatalf("failed to subscribe to RedisBus: %v", err)
	}

	// Give subscription a moment to initialize in Redis
	time.Sleep(100 * time.Millisecond)

	publishedMsg := &service.OutMessage{
		Type:           "message",
		ID:             msgID,
		SenderID:       uuid.New(),
		SenderUsername: "pubsub_tester",
		ConversationID: convID,
		Content:        testContent,
		CreatedAt:      time.Now().UTC(),
	}

	// Publish message
	if err := bus.Publish(ctx, publishedMsg); err != nil {
		t.Fatalf("failed to publish message via RedisBus: %v", err)
	}

	// Wait for message delivery via subscriber channel
	select {
	case receivedMsg := <-receivedChan:
		if receivedMsg.Content != testContent {
			t.Fatalf("expected message content %q, got %q", testContent, receivedMsg.Content)
		}
		if receivedMsg.ConversationID != convID {
			t.Fatalf("expected conversation ID %s, got %s", convID, receivedMsg.ConversationID)
		}
		t.Logf("Successfully received published message over Redis Pub/Sub: ID=%s, Content=%s", receivedMsg.ID, receivedMsg.Content)
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for message delivery over Redis Pub/Sub")
	}
}
