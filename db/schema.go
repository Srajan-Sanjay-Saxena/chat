package db

import(
	"github.com/google/uuid"
	"time"
)

type User struct {
	ID uuid.UUID `json:"id"`
	Username string `json:"username"`
	PasswordHash string `json:"-"`
	Email string `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt int64 `json:"updated_at"`
}

type Message struct {
	ID uuid.UUID `json:"id"`
	SenderID uuid.UUID `json:"sender_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content string `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt int64 `json:"updated_at"`
	// is_deleted bool `json:"is_deleted"`
}

type Conversation struct {
	ID uuid.UUID `json:"id"`
	Title string `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt int64 `json:"updated_at"`
	// CreatedBy User `json:"created_by"`
}

type ConversationParticipant struct {
	ID uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	UserID uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}