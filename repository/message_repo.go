package repository

import (
	"chat-v2/db"
	"chat-v2/logger"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type message struct {
	ID             uuid.UUID `json:"id"`
	SenderID       uuid.UUID `json:"sender_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

type messageWithUsername struct {
	ID             uuid.UUID `json:"id"`
	SenderID       uuid.UUID `json:"sender_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
	SenderUsername string    `json:"sender_username"`
}

type MessageResponse struct {
	Messages   []*messageWithUsername
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
	start := time.Now()
	// Writing SQL query to insert a new message into the database
	// query := `insert into messages (id, conversation_id, sender_id, content, created_at) values ($1, $2, $3, $4, $5) returning id, created_at`
	query := fmt.Sprintf(`
	insert into %s (id, conversation_id, sender_id, content, created_at) values ($1, $2, $3, $4, $5) 
	returning id, created_at`, r.table("messages"))

	// Executing the query and scanning the returned id into the message struct
	err := r.DB.QueryRow(ctx, query, message.ID, message.ConversationID, message.SenderID, message.Content, message.CreatedAt).Scan(&message.ID, &message.CreatedAt)
	logger.Log.Debug("CreateMessage query executed", "message_id", message.ID, "conversation_id", message.ConversationID, "sender_id", message.SenderID, "duration_ms", time.Since(start).Milliseconds())
	return err
}

func (r *Repository) GetMessageByID(ctx context.Context, id uuid.UUID) (*db.Message, error) {

	// Writing SQL query to select a message by id
	// query := `select * from messages where id = $1`
	query := fmt.Sprintf(`select * from %s where id = $1`, r.table("messages"))

	// Executing the query and scanning the result into a message struct
	var message db.Message
	err := r.DB.QueryRow(ctx, query, id).Scan(&message.ID, &message.SenderID, &message.ConversationID, &message.Content, &message.CreatedAt)
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
	args := []any{}
	idx := 1
	query := fmt.Sprintf(`
		select m.id, m.conversation_id, m.content, m.created_at, u.username, m.sender_id
		from %s m
		join %s u on m.sender_id = u.id
		where m.conversation_id = $1
	`, r.table("messages"), r.table("users"))
	// query := fmt.Sprintf(` select m.id, m.conversation_id, m.content, m.created_at, m.sender_id from %s m where m.conversation_id = $1`, r.table("messages"))

	args = append(args, conversationID)
	idx++

	if before != nil {
		cursor, err := decodeCursor(*before)
		if err != nil {
			return nil, err
		}
		// use two placeholders for created_at and id
		query += fmt.Sprintf(" and (m.created_at < $%d or (m.created_at = $%d and m.id < $%d))", idx, idx, idx+1)
		args = append(args, cursor.CreatedAt, cursor.ID)
		idx += 2
	}

	// Fetch one extra row to determine if there are more pages
	fetchLimit := limit + 1
	query += fmt.Sprintf(" order by m.created_at desc, m.id desc limit $%d", idx)
	args = append(args, fetchLimit)

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*messageWithUsername
	for rows.Next() {
		var msg messageWithUsername
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Content, &msg.CreatedAt, &msg.SenderUsername, &msg.SenderID); err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
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
	}

	// Reverse messages to return in ascending order (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return &MessageResponse{Messages: messages, NextCursor: encodedNext, HasMore: hasMore}, nil
}
