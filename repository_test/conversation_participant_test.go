package repository_test

import (
	"chat-v2/db"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestConversationParticipantRepo(t *testing.T) {
	resetTestDatabase(t)

	suffix := uuid.NewString()
	user := &db.User{
		ID:           uuid.New(),
		Username:     "participant_user_" + suffix,
		PasswordHash: "hash",
		Email:        "participant_user_" + suffix + "@example.com",
		CreatedAt:    time.Now().UTC(),
	}
	if err := repo.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	conversation := &db.Conversation{
		ID:        uuid.New(),
		Title:     "participant conversation " + uuid.NewString(),
		CreatedAt: time.Now().UTC(),
	}
	if err := repo.CreateConversation(context.Background(), conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	if err := repo.AddParticipant(context.Background(), conversation.ID, user.ID); err != nil {
		t.Fatalf("add participant: %v", err)
	}

	isParticipant, err := repo.IsParticipant(context.Background(), conversation.ID, user.ID)
	if err != nil {
		t.Fatalf("is participant: %v", err)
	}
	if !isParticipant {
		t.Fatalf("expected user to be participant")
	}

	participants, err := repo.GetParticipantsByConversationID(context.Background(), conversation.ID)
	if err != nil {
		t.Fatalf("get participants: %v", err)
	}
	if len(participants) != 1 || participants[0] != user.ID {
		t.Fatalf("unexpected participants: %#v", participants)
	}

	if err := repo.RemoveParticipant(context.Background(), conversation.ID, user.ID); err != nil {
		t.Fatalf("remove participant: %v", err)
	}

	isParticipant, err = repo.IsParticipant(context.Background(), conversation.ID, user.ID)
	if err != nil {
		t.Fatalf("is participant after remove: %v", err)
	}
	if isParticipant {
		t.Fatalf("expected user to be removed from participants")
	}
}
