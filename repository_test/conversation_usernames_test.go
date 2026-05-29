package repository_test

import (
	"chat-v2/db"
	"chat-v2/helper"
	"chat-v2/repository"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateConversationByUsernames_Dedupe(t *testing.T) {
	// reset database
	if err := helper.ResetSchema(); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	repo := repository.NewRepository(db.DB)

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
