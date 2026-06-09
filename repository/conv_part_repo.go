package repository

import (
	"context"
	"time"
	"chat-v2/logger"
	"github.com/google/uuid"
	"fmt"
)

func (r *Repository) AddParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {

	// Writing SQL query to insert a new participant into the conversation_participants table
	// query := `insert into conversation_participants (conversation_id, user_id) values ($1, $2)`, 
	query := fmt.Sprintf(`insert into %s (conversation_id, user_id) values ($1, $2)`, r.table("conversation_participants"))

	// Executing the query
	_, err := r.DB.Exec(ctx, query, conversationID, userID)
	return err
}

func (r *Repository) RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {

	// Writing SQL query to delete a participant from the conversation_participants table
	// query := `delete from conversation_participants where conversation_id = $1 and user_id = $2`
	query := fmt.Sprintf(`delete from %s where conversation_id = $1 and user_id = $2`, r.table("conversation_participants"))

	// Executing the query
	_, err := r.DB.Exec(ctx, query, conversationID, userID)
	return err
}

func (r *Repository) GetParticipantsByConversationID(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {

	// Writing SQL query to select user ids of participants in a conversation
	// query := `select user_id from conversation_participants where conversation_id = $1`
	query := fmt.Sprintf(`select user_id from %s where conversation_id = $1`, r.table("conversation_participants"))

	// Executing the query and scanning the results into a slice of user ids
	rows, err := r.DB.Query(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

func (r *Repository) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	start := time.Now()
	var exists bool
	// query := `select exists(select 1 from conversation_participants where conversation_id=$1 and user_id=$2)`
	query := fmt.Sprintf(`select exists(select 1 from %s where conversation_id=$1 and user_id=$2)`, r.table("conversation_participants"))

	err := r.DB.QueryRow(ctx, query, conversationID, userID).Scan(&exists)
	logger.Log.Debug("IsParticipant query executed", "conversation_id", conversationID, "user_id", userID, "exists", exists, "duration_ms", time.Since(start).Milliseconds())
	return exists, err
}

func (r *Repository) GetFriends(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	// query := `
	//     SELECT 
	// 	u.id, 
	// 	u.username, 
	// 	u.email, 
	// 	u.created_at
	// 	FROM conversation_participant cp1
	// 	JOIN conversation c ON cp1.conversation_id = c.id
	// 	JOIN conversation_participant cp2 ON c.id = cp2.conversation_id
	// 	JOIN users u ON cp2.user_id = u.id
	// 	WHERE cp1.user_id = $1            
	// 	AND c.type = 'private'          
	// 	AND cp2.user_id != $1;          
	// `

	query := fmt.Sprintf(`
	SELECT 
	u.id, 
	u.username, 
	u.email, 
	u.created_at
	FROM %s cp1
	JOIN %s c ON cp1.conversation_id = c.id
	JOIN %s cp2 ON c.id = cp2.conversation_id
	JOIN %s u ON cp2.user_id = u.id
	WHERE cp1.user_id = $1            
	AND c.type = 'private'          
	AND cp2.user_id != $1;          
	`, r.table("conversation_participants"), r.table("conversations"), r.table("conversation_participants"), r.table("users"))
	
	rows, err := r.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var friends []uuid.UUID
	for rows.Next() {
		var friendID uuid.UUID
		if err := rows.Scan(&friendID); err != nil {
			return nil, err
		}
		friends = append(friends, friendID)
	}
	return friends, nil
}
