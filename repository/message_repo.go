package repository

import (
	"chat-v2/db"
	"chat-v2/logger"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"time"
)

type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type MessageResponse struct {
	Messages   []*db.Message
	NextCursor string
	HasMore    bool
}

func encodeCursor(c Cursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

func decodeCursor(s string) (*Cursor, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	var c Cursor

	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}

	return &c, nil
}

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

func (r *Repository) GetMessagesByConversationID(ctx context.Context, conversationID uuid.UUID, before *string, limit int) (*MessageResponse, error) {

	// limit check
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	// Build query with positional placeholders safely
	args := []interface{}{}
	idx := 1
	query := fmt.Sprintf("select id, conversation_id, sender_id, content, created_at from messages where conversation_id = $%d", idx)
	args = append(args, conversationID)
	idx++

	if before != nil {
		cursor, err := decodeCursor(*before)
		if err != nil {
			return nil, err
		}
		// use two placeholders for created_at and id
		query += fmt.Sprintf(" and (created_at < $%d or (created_at = $%d and id < $%d))", idx, idx, idx+1)
		args = append(args, cursor.CreatedAt, cursor.ID)
		idx += 2
	}

	// Fetch one extra row to determine if there are more pages
	fetchLimit := limit + 1
	query += fmt.Sprintf(" order by created_at desc, id desc limit $%d", idx)
	args = append(args, fetchLimit)

	rows, err := r.DB.Query(ctx, query, args...)
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

	hasMore := false
	if len(messages) > limit {
		hasMore = true
		messages = messages[:limit]
	}

	var encodedNext string
	if hasMore && len(messages) > 0 {
		lastMessage := messages[len(messages)-1]
		c, err := encodeCursor(Cursor{CreatedAt: lastMessage.CreatedAt, ID: lastMessage.ID})
		if err != nil {
			return nil, err
		}
		encodedNext = c
		logger.Log.Info("Next cursor for pagination", "cursor", c)
	}

	return &MessageResponse{Messages: messages, NextCursor: encodedNext, HasMore: hasMore}, nil
}
