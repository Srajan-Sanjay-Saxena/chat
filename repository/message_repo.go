package repository

import (
	"chat-v2/db"
	"context"
	"github.com/google/uuid"
)

func (r *Repository) CreateMessage(ctx context.Context, message *db.Message) error {

	// Writing SQL query to insert a new message into the database
	query := `insert into messages (id, conversation_id, sender_id, content, created_at) values ($1, $2, $3, $4, $5) returning id, created_at`

	// Executing the query and scanning the returned id into the message struct
	return r.DB.QueryRow(ctx, query, message.ID, message.ConversationID, message.SenderID, message.Content, message.CreatedAt).Scan(&message.ID, &message.CreatedAt)
}

func (r *Repository) GetMessageByID(ctx context.Context, id uuid.UUID) (*db.Message, error) {

	// Writing SQL query to select a message by id
	query := `select * from messages where id = $1`

	// Executing the query and scanning the result into a message struct
	var message db.Message
	err := r.DB.QueryRow(ctx, query, id).Scan(&message.ID, &message.ConversationID, &message.SenderID, &message.Content, &message.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *Repository) GetMessagesByConversationID(ctx context.Context, conversationID uuid.UUID) ([]*db.Message, error) {
	// Writing SQL query to select messages by conversation id
	query := `select * from messages where conversation_id = $1 order by created_at`

	// Executing the query and scanning the results into a slice of message structs
	rows, err := r.DB.Query(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*db.Message
	for rows.Next() {
		var message db.Message
		if err := rows.Scan(&message.ID, &message.ConversationID, &message.SenderID, &message.Content, &message.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, &message)
	}
	return messages, nil
}
