package ws_test

import (
	"chat-v2/config"
	"chat-v2/db"
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"chat-v2/ws"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"log"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testRepo *repository.Repository
var suiteLockConn *pgxpool.Conn
var DB *pgxpool.Pool
const suiteLockKey int64 = 842020
var maker *helper.JWTMaker
// func resetTestDatabase(t *testing.T) {
// 	t.Helper()
// 	if err := helper.ResetSchema(DB); err != nil {
// 		t.Fatalf("failed to reset database schema: %v", err)
// 	}
// }

func TestMain(m *testing.M) {
	logger.TestInit()
	logger.Log.Info("Starting test suite setup")

	cfg, err := config.LoadConfig("../.env")
	if err != nil {
		log.Fatalf("configuration loading failed: %v", err)
	}

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	DB , err := db.Connect(cfg.DBSource)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() {
		DB.Close()
	} ()
	
	_, err = DB.Exec(
		context.Background(),
		fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema),
	)
	if err != nil {
		log.Fatalf("failed to create schema: %v", err)
	}

	if err := helper.ResetSchema(DB, schema); err != nil {
		log.Fatalf("failed to reset database schema: %v", err)
	}
	testRepo, err = repository.NewRepository(DB, schema)
	if err != nil {
		log.Fatalf("failed to initialize repository: %v", err)
	}
	logger.Log.Info("Database setup complete")

	maker, err = helper.NewJWTMaker(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("failed to create JWT maker: %v", err)
	}

	// redis connection and presence store creation
	// redisAddr := os.Getenv("REDIS_ADDR")
	// redisPassword := os.Getenv("REDIS_PASSWORD")
	// redisUsername := os.Getenv("REDIS_USERNAME")
	// redisDB := 0

	// redisClient , err := redis.Connect(redisAddr, redisUsername, redisPassword, redisDB)
	// if err != nil {
	// 	panic("failed to connect to Redis: " + err.Error())
	// }
	// defer redisClient.Close()

	// presenceStore = redis.NewPresenceStore(redisClient)

	exitCode := m.Run()

	_, err = DB.Exec(
		context.Background(),
		fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema),
	)
	if err != nil {
		logger.Log.Error("drop schema", "err", err)
	}
	os.Exit(exitCode)
}

type wsInMessage struct {
	Type           string    `json:"type"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
}

type wsOutMessage struct {
	Type           string    `json:"type"`
	ID             uuid.UUID `json:"id"`
	SenderID       uuid.UUID `json:"sender_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

func TestWebSocketIntegration_PublishSubscribePersist(t *testing.T) {
	// resetTestDatabase(t)

	userID := uuid.New()
	conversationID := uuid.New()
	allowedOrigin := "https://app.example.com"

	user := &db.User{
		ID:           userID,
		Username:     "ws_user_" + uuid.NewString(),
		PasswordHash: "hash",
		Email:        "ws_user_" + uuid.NewString() + "@example.com",
		CreatedAt:    time.Now().UTC(),
	}
	if err := testRepo.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	conversation := &db.Conversation{
		ID:        conversationID,
		Title:     "ws test conversation " + uuid.NewString(),
		CreatedAt: time.Now().UTC(),
	}
	if err := testRepo.CreateConversation(context.Background(), conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := testRepo.AddParticipant(context.Background(), conversationID, userID); err != nil {
		t.Fatalf("add participant: %v", err)
	}

	token, err := maker.CreateToken(userID, time.Hour)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	hub := ws.NewHub(nil)
	go hub.Run()
	defer func() {
		hub.Stop()
		<-hub.Done()
	}()

	mux := http.NewServeMux()
	mux.Handle("/ws", ws.NewWebSocketHandler(testRepo, hub, maker, config.ParseAllowedOrigins(allowedOrigin), testRepo.IsParticipant))
	server := httptest.NewServer(mux)
	defer server.Close()

	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Set("Origin", allowedOrigin)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws" + "?token=" + token
	conn, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			t.Fatalf("dial websocket: %v (status=%d)", err, resp.StatusCode)
		}
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
	defer func() {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}()

	readDeadline := time.Now().Add(10 * time.Second)
	if err := conn.SetReadDeadline(readDeadline); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	if err := conn.WriteJSON(wsInMessage{Type: "subscribe", ConversationID: conversationID}); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	var ack map[string]any
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatalf("read subscribe ack: %v", err)
	}
	if ack["type"] != "subscribed" {
		t.Fatalf("unexpected subscribe ack: %#v", ack)
	}

	content := "hello from websocket integration test"
	if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatalf("set read deadline before publish: %v", err)
	}
	if err := conn.WriteJSON(wsInMessage{Type: "message", ConversationID: conversationID, Content: content}); err != nil {
		t.Fatalf("write message: %v", err)
	}

	var msg wsOutMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read broadcast message: %v", err)
	}
	if msg.Type != "message" {
		t.Fatalf("unexpected message type: %#v", msg)
	}
	if msg.SenderID != userID || msg.ConversationID != conversationID || msg.Content != content {
		t.Fatalf("broadcast payload mismatch: %#v", msg)
	}
	if msg.ID == uuid.Nil {
		t.Fatalf("expected persisted id, got nil")
	}
	if msg.CreatedAt.IsZero() {
		t.Fatalf("expected created_at to be populated")
	}

	msgResp, err := testRepo.GetMessagesByConversationID(context.Background(), conversationID, nil, 10)
	if err != nil {
		t.Fatalf("fetch persisted messages: %v", err)
	}
	if len(msgResp.Messages) != 1 {
		t.Fatalf("expected 1 persisted message, got %d", len(msgResp.Messages))
	}
	if msgResp.Messages[0].ID != msg.ID || msgResp.Messages[0].Content != content || msgResp.Messages[0].SenderID != userID {
		t.Fatalf("persisted row mismatch: got=%+v want=%+v", msgResp.Messages[0], msg)
	}

	// Verify pagination metadata for single-page result
	if msgResp.NextCursor != "" {
		t.Fatalf("expected empty NextCursor for single page, got=%#v", msgResp.NextCursor)
	}
	if msgResp.HasMore {
		t.Fatalf("expected HasMore=false for single page, got=true")
	}

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err == nil {
		_, _, _ = conn.ReadMessage()
	}
}
