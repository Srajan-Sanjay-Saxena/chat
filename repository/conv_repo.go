package repository

import (
	"chat-v2/db"
	"context"
	"time"
	"github.com/google/uuid"
)

func (r *Repository) CreateConversation(ctx context.Context, conversation *db.Conversation) error {
	// Writing SQL query to insert a new conversation into the database
	query := `insert into conversations (id, title, created_at) values ($1, $2, $3) returning id`

	// Executing the query and scanning the returned id into the conversation struct
	return r.DB.QueryRow(ctx, query, conversation.ID, conversation.Title, conversation.CreatedAt).Scan(&conversation.ID)
}

func (r *Repository) GetConversationByID(ctx context.Context, id uuid.UUID) (*db.Conversation, error) {
	// Writing SQL query to select a conversation by id
	query := `select * from conversations where id = $1`

	// Executing the query and scanning the result into a conversation struct
	var conversation db.Conversation
	err := r.DB.QueryRow(ctx, query, id).Scan(&conversation.ID, &conversation.Title, &conversation.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (r *Repository) GetConversationsByUserID(ctx context.Context, userID uuid.UUID) ([]*db.Conversation, error) {
	// Writing SQL query to select conversations by user id
	query := `select c.id, c.title, c.created_at from conversations c 
			  join conversation_participants cp on c.id = cp.conversation_id 
			  where cp.user_id = $1 order by c.created_at`

	// Executing the query and scanning the results into a slice of conversation structs
	rows, err := r.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []*db.Conversation
	for rows.Next() {
		var conversation db.Conversation
		if err := rows.Scan(&conversation.ID, &conversation.Title, &conversation.CreatedAt); err != nil {
			return nil, err
		}
		conversations = append(conversations, &conversation)
	}
	return conversations, nil
}

func (r *Repository) CreateConversationWithParticipants(ctx context.Context, title string, participantIDs []uuid.UUID) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		} else {
			tx.Commit(ctx)
		}
	}()

	conversation := &db.Conversation{
		ID:        uuid.New(),
		Title:     title,
		CreatedAt: time.Now(),
	}

	if err := r.CreateConversation(ctx, conversation); err != nil {
		return err
	}
	return nil
}
