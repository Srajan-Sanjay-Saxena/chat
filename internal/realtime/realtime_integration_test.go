package realtime

import (
	"chat-v2/config"
	"chat-v2/db"
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"chat-v2/service"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"chat-v2/middleware"
)

var testRepo *repository.Repository
var DB *db.Db
var maker *helper.JWTMaker

func TestMain(m *testing.M) {
	logger.TestInit()
	logger.Log.Info("Starting Hub tests...")

	cfg, err := config.LoadConfig("../../.env")
	if err != nil {
		logger.Log.Error("configuration loading failed", "error", err)
	}

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	DB, err := db.Connect(cfg.DBSource)
	if err != nil {
		logger.Log.Error("failed to connect to database", "error", err)
	}
	defer func() {
		DB.Close()
	}()

	_, err = DB.Exec(
		context.Background(),
		fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema),
	)
	if err != nil {
		logger.Log.Error("failed to create schema", "error", err)
	}

	if err := helper.ResetSchema(DB, schema); err != nil {
		logger.Log.Error("failed to reset database schema", "error", err)
	}

	testRepo, err = repository.NewRepository(DB, schema)
	if err != nil {
		logger.Log.Error("failed to create repository", "error", err)
	}

	maker, err = helper.NewJWTMaker(cfg.JWTSecret)
	if err != nil {
		logger.Log.Error("failed to create JWT maker", "error", err)
	}

	logger.Log.Info("Test suite setup completed successfully")

	exitCode := m.Run()
	_, err = DB.Exec(
		context.Background(),
		fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema),
	)
	if err != nil {
		logger.Log.Error("failed to drop test schema", "error", err)
	}
	os.Exit(exitCode)
}

type wsIncomingMessage struct {
	Type           string    `json:"type"`
	Content        string    `json:"content,omitempty"`
	ConversationID uuid.UUID `json:"conversation_id"`
}

type wsOutgoingMessage struct {
	Type           string    `json:"type"`
	Content        string    `json:"content,omitempty"`
	ConversationID uuid.UUID `json:"conversation_id,omitempty"`
	SenderID       uuid.UUID `json:"sender_id,omitempty"`
	SenderUsername string    `json:"sender_username,omitempty"`
	ID             uuid.UUID `json:"id,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
}

func TestWSIntegration(t *testing.T) {
	userID := uuid.New()
	convID := uuid.New()
	username := "testuser"
	msgContent := "Hello, World!"
	allowedOrigin := "https://example.com"
	user := &db.User{
		ID:           userID,
		Username:     username,
		Email:        fmt.Sprintf("%s@example.com", username),
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := testRepo.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	conv := &db.Conversation{
		ID:        convID,
		Title:     "Test Conversation",
		CreatedAt: time.Now(),
	}
	if err := testRepo.CreateConversation(context.Background(), conv); err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}
	if err := testRepo.AddParticipant(context.Background(), convID, userID); err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}

	token, err := maker.CreateToken(userID, time.Hour)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	hub := NewHub()
	go hub.Run()
	defer func() {
		hub.Stop()
		<-hub.Done()
	}()

	publisher := NewLocalPublisher(hub)
	subscriptionService := service.NewSubscriptionService(testRepo)
	messageService := service.NewMessageService(testRepo, publisher)
	realtimeHandler := NewRealtimeHandler(hub, subscriptionService, messageService)
	authMiddleware := middleware.JWTMiddleware(maker)
	mux := http.NewServeMux()
	mux.Handle("/ws", authMiddleware(NewWSHandler(realtimeHandler, maker, []string{allowedOrigin})) )
	server := httptest.NewServer(mux)
	defer server.Close()

	dialer := websocket.Dialer{}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	header := http.Header{}
	header.Set("Origin", allowedOrigin)
	header.Add("Cookie", fmt.Sprintf("access_token=%s", token))
	conn, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			t.Fatalf("failed to connect to WebSocket: %v, status code: %d", err, resp.StatusCode)
		} else {
			t.Fatalf("failed to connect to WebSocket: %v", err)
		}
	}
	defer conn.Close()
	defer func() {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}()

	if err := conn.WriteJSON(wsIncomingMessage{
		Type:           "subscribe",
		ConversationID: convID,
	}); err != nil {
		t.Fatalf("failed to send subscribe message: %v", err)
	}

	var ack map[string]any
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatalf("failed to read subscribe ack: %v", err)
	}
	if ack["type"] != "subscribe_ack" {
		t.Fatalf("unexpected subscribe ack: %#v", ack)
	}

	if err := conn.WriteJSON(wsIncomingMessage{
		Type:           "message",
		Content:        msgContent,
		ConversationID: convID,
	}); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
	var outMsg wsOutgoingMessage
	if err := conn.ReadJSON(&outMsg); err != nil {
		t.Fatalf("failed to read outgoing message: %v", err)
	}
	if outMsg.Type != "message" {
		t.Fatalf("unexpected outgoing message type: %#v", outMsg)
	}
	if msgContent != outMsg.Content {
		t.Fatalf("unexpected outgoing message content: got %q, want %q", outMsg.Content, msgContent)
	}

	msgRecord, err := testRepo.GetMessageByID(context.Background(), outMsg.ID)
	if err != nil {
		t.Fatalf("failed to retrieve message from database: %v", err)
	}
	if msgRecord == nil {
		t.Fatalf("message record not found in database")
	}

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err == nil {
		_, _, err := conn.ReadMessage()
		if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
			t.Fatalf("unexpected error reading from WebSocket after closure: %v", err)
		}
	}

}
