package repository

import (
	"chat-v2/db"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNilDB = errors.New("database connection pool cannot be nil")

type UserRepository interface {
	CreateUser(ctx context.Context, user *db.User) error
	GetUserByID(ctx context.Context, id uuid.UUID) (*db.User, error)
	GetUserByUsername(ctx context.Context, username string) (*db.User, error)
	SearchUsers(ctx context.Context, q string, limit int) ([]*db.User, error)
}

type MessageRepository interface {
	CreateMessage(ctx context.Context, message *db.Message) error
	GetMessageByID(ctx context.Context, id uuid.UUID) (*db.Message, error)
	GetMessagesByConversationID(ctx context.Context, conversationID uuid.UUID, before *string, limit int) (*MessageResponse, error)
}

type ConversationRepository interface {
	CreateConversation(ctx context.Context, conversation *db.Conversation) error
	GetConversationByID(ctx context.Context, id uuid.UUID) (*db.Conversation, error)
	GetConversationsByUserID(ctx context.Context, userID uuid.UUID) ([]*db.Conversation, error)
	GetConversationByCanonicalName(ctx context.Context, canonical string) (*db.Conversation, error)
}

type ConversationParticipantRepository interface {
	AddParticipant(ctx context.Context, conversationID, userID uuid.UUID) error
	RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) error
	GetParticipantsByConversationID(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error)
}

type Repository struct {
	DB *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) (*Repository, error) {

	if db == nil {
		return nil, ErrNilDB
	}

	return &Repository{
		DB: db,
	}, nil
}
