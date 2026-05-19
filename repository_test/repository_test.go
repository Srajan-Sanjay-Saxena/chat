package repository_test

import (
	"chat-v2/db"
	"chat-v2/logger"
	"chat-v2/repository"
	"context"
	"os"
	"testing"
	"github.com/google/uuid"
	"time"
	"github.com/joho/godotenv"
	"chat-v2/helper"
)

var repo *repository.Repository

func TestMain(m *testing.M) {
	// Load .env file
	godotenv.Load("../.env")

	// Load logger 
	logger.Init()

	// Connect to the database
	dsn := os.Getenv("dbSource")
	if err := db.Connect(dsn); err != nil {
		panic("Failed to connect to database: " + err.Error())
	}

	if err := db.DB.Ping(context.Background()); err != nil {
		panic("Failed to ping database: " + err.Error())
	}	

	if err := helper.Rollback(); err != nil {
		panic("Failed to rollback database: " + err.Error())
	}
	
	if err := helper.Migrate(); err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	repo = repository.NewRepository(db.DB)
	// Run tests

	os.Exit(m.Run())
}

func TestUserRepo(t *testing.T) {
	// Create a new user
	user := &db.User{
		ID:           uuid.New(),
		Username:     "testuser",
		PasswordHash: "password",
		Email:        "test1@gmail.com",
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
	// Create a new conversation
	conversation := &db.Conversation{
		ID:        uuid.New(),
		Title:     "Test Conversation",
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
	// Creste users
	user1 := &db.User{
		ID:           uuid.New(),
		Username:     "testuser1",
		PasswordHash: "password",
		Email:        "test2@gmail.com",
		CreatedAt:    time.Now(),
	}

	user2 := &db.User{
		ID:           uuid.New(),
		Username:     "testuser2",
		PasswordHash: "password",
		Email:        "test3@gmail.com",
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
		Title:     "Test Conversation",
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
		Content:        "Hello, world!",
		CreatedAt:      time.Now(),
	}
	err = repo.CreateMessage(context.Background(), message)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	// Get messages by conversation ID
	messages, err := repo.GetMessagesByConversationID(context.Background(), conversation.ID)
	if err != nil {
		t.Fatalf("Failed to get messages by conversation ID: %v", err)
	}

	if len(messages) != 1 || (messages[0].ID != message.ID || messages[0].Content != message.Content || messages[0].SenderID != message.SenderID) {
		t.Errorf("Fetched messages do not match created message")
	}
}