package repository_test

import (
	"chat-v2/db"
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"context"
	"fmt"
	"os"
	"testing"
	"time"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"chat-v2/config"
	"log"
)

var repo *repository.Repository
var DB *pgxpool.Pool

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

	repo, err = repository.NewRepository(DB, schema)
	if err != nil {
		log.Fatalf("failed to initialize repository: %v", err)
	}
	logger.Log.Info("Database setup complete")

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

func TestUserRepo(t *testing.T) {
	// resetTestDatabase(t)

	// Create a new user
	suffix := uuid.NewString()
	user := &db.User{
		ID:           uuid.New(),
		Username:     "testuser_" + suffix,
		PasswordHash: "password",
		Email:        "test1_" + suffix + "@gmail.com",
		CreatedAt:    time.Now(),
	}
	err := repo.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Get the user by ID
	fetched, err := repo.GetUserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to get user by ID: %v", err)
	}
	if fetched.ID != user.ID || fetched.Username != user.Username || fetched.Email != user.Email {
		t.Errorf("fetched user mismatch: got=%+v want=%+v", fetched, user)
	}

	// Get the user by username
	fetched, err = repo.GetUserByUsername(context.Background(), user.Username)
	if err != nil {
		t.Fatalf("Failed to get user by username: %v", err)
	}
	if fetched.ID != user.ID || fetched.Username != user.Username || fetched.Email != user.Email {
		t.Errorf("fetched user mismatch: got=%+v want=%+v", fetched, user)
	}
}

func TestConversationRepo(t *testing.T) {
	// resetTestDatabase(t)

	// Create a new conversation
	conversation := &db.Conversation{
		ID:        uuid.New(),
		Title:     "Test Conversation " + uuid.NewString(),
		CreatedAt: time.Now(),
	}
	err := repo.CreateConversation(context.Background(), conversation)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Get the conversation by ID
	fetched, err := repo.GetConversationByID(context.Background(), conversation.ID)
	if err != nil {
		t.Fatalf("Failed to get conversation by ID: %v", err)
	}

	if fetched.ID != conversation.ID || fetched.Title != conversation.Title {
		t.Errorf("fetched conversation mismatch: got=%+v want=%+v", fetched, conversation)
	}

}

func TestFlow(t *testing.T) {
	// resetTestDatabase(t)

	// Creste users
	userSuffix := uuid.NewString()
	user1 := &db.User{
		ID:           uuid.New(),
		Username:     "testuser1_" + userSuffix,
		PasswordHash: "password",
		Email:        "test2_" + userSuffix + "@gmail.com",
		CreatedAt:    time.Now(),
	}

	user2 := &db.User{
		ID:           uuid.New(),
		Username:     "testuser2_" + userSuffix,
		PasswordHash: "password",
		Email:        "test3_" + userSuffix + "@gmail.com",
		CreatedAt:    time.Now(),
	}

	err := repo.CreateUser(context.Background(), user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}
	err = repo.CreateUser(context.Background(), user2)
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Create a conversation
	conversation := &db.Conversation{
		ID:        uuid.New(),
		Title:     "Test Conversation " + uuid.NewString(),
		CreatedAt: time.Now(),
	}
	err = repo.CreateConversation(context.Background(), conversation)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	err = repo.AddParticipant(context.Background(), conversation.ID, user1.ID)
	if err != nil {
		t.Fatalf("Failed to add participant user1: %v", err)
	}

	err = repo.AddParticipant(context.Background(), conversation.ID, user2.ID)
	if err != nil {
		t.Fatalf("Failed to add participant user2: %v", err)
	}

	// Create a message
	message := &db.Message{
		ID:             uuid.New(),
		ConversationID: conversation.ID,
		SenderID:       user1.ID,
		SenderUsername: user1.Username,
		Content:        "Hello, world!",
		CreatedAt:      time.Now(),
	}
	err = repo.CreateMessage(context.Background(), message)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	// Get messages by conversation ID
	msgResp, err := repo.GetMessagesByConversationID(context.Background(), conversation.ID, nil, 10)
	if err != nil {
		t.Fatalf("Failed to get messages by conversation ID: %v", err)
	}

	if len(msgResp.Messages) != 1 || (msgResp.Messages[0].ID != message.ID || msgResp.Messages[0].Content != message.Content || msgResp.Messages[0].SenderID != message.SenderID || msgResp.Messages[0].SenderUsername != message.SenderUsername) {
		t.Errorf("Fetched messages do not match created message")
	}

	// Pagination metadata expectations for a single-page result
	if msgResp.NextCursor != "" {
		t.Errorf("expected empty NextCursor for single page, got=%#v", msgResp.NextCursor)
	}
	if msgResp.HasMore {
		t.Errorf("expected HasMore=false for single page, got=true")
	}
}

func TestMessagePaginationCursor(t *testing.T) {
	// resetTestDatabase(t)

	// Setup: create user and conversation
	suffix := uuid.NewString()
	user := &db.User{
		ID:           uuid.New(),
		Username:     "pag_user_" + suffix,
		PasswordHash: "pwd",
		Email:        "pag_user_" + suffix + "@example.com",
		CreatedAt:    time.Now(),
	}
	if err := repo.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	conversation := &db.Conversation{
		ID:        uuid.New(),
		Title:     "pagination convo " + uuid.NewString(),
		CreatedAt: time.Now(),
	}
	if err := repo.CreateConversation(context.Background(), conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := repo.AddParticipant(context.Background(), conversation.ID, user.ID); err != nil {
		t.Fatalf("add participant: %v", err)
	}

	// Insert 25 messages (newest first: i=0 newest)
	total := 25
	for i := 0; i < total; i++ {
		m := &db.Message{
			ID:             uuid.New(),
			ConversationID: conversation.ID,
			SenderID:       user.ID,
			SenderUsername: user.Username,
			Content:        fmt.Sprintf("msg %d", i),
			CreatedAt:      time.Now().Add(time.Duration(-i) * time.Second),
		}
		if err := repo.CreateMessage(context.Background(), m); err != nil {
			t.Fatalf("create message %d: %v", i, err)
		}
	}

	// Page through with limit=10
	var seen int
	var before *string
	limit := 10
	for {
		resp, err := repo.GetMessagesByConversationID(context.Background(), conversation.ID, before, limit)
		if err != nil {
			t.Fatalf("get messages page: %v", err)
		}
		seen += len(resp.Messages)

		if resp.HasMore {
			if resp.NextCursor == "" {
				t.Fatalf("HasMore=true but NextCursor is empty")
			}
			// NextCursor is already encoded; use it directly for the next request
			before = &resp.NextCursor
			continue
		}
		break
	}

	if seen != total {
		t.Fatalf("expected to see %d messages after pagination, saw %d", total, seen)
	}
}

func TestCreateConversationByUsernames_Dedupe(t *testing.T) {

	// create two users
	u1 := &db.User{ID: uuid.New(), Username: "u1_" + uuid.NewString()[:6], PasswordHash: "h", Email: uuid.NewString() + "@example.com", CreatedAt: time.Now()}
	u2 := &db.User{ID: uuid.New(), Username: "u2_" + uuid.NewString()[:6], PasswordHash: "h", Email: uuid.NewString() + "@example.com", CreatedAt: time.Now()}
	if err := repo.CreateUser(context.Background(), u1); err != nil {
		t.Fatalf("create user1: %v", err)
	}
	if err := repo.CreateUser(context.Background(), u2); err != nil {
		t.Fatalf("create user2: %v", err)
	}

	// canonical name is lowercased lexicographic order
	a := u1.Username
	b := u2.Username
	if a > b {
		a, b = b, a
	}
	canonical := a + ":" + b

	conv := &db.Conversation{ID: uuid.New(), Type: "private", CanonicalName: canonical, CreatedAt: time.Now()}

	// create conversation by usernames
	if err := repo.CreateConversationWithParticipantsByUsernames(context.Background(), conv, []string{u1.Username, u2.Username}); err != nil {
		t.Fatalf("create conversation by usernames: %v", err)
	}
	
	// attempt to create same conversation again - should return ErrConversationExists
	conv2 := &db.Conversation{ID: uuid.New(), Type: "private", CanonicalName: canonical, CreatedAt: time.Now()}
	if err := repo.CreateConversationWithParticipantsByUsernames(context.Background(), conv2, []string{u1.Username, u2.Username}); err == nil {
		t.Fatalf("expected duplicate creation to fail, got nil")
	}

	// verify conversation is returned by canonical lookup
	got, err := repo.GetConversationByCanonicalName(context.Background(), canonical)
	if err != nil {
		t.Fatalf("get by canonical: %v", err)
	}
	if got == nil || got.CanonicalName != canonical {
		t.Fatalf("unexpected conversation from canonical lookup: %#v", got)
	}
}
