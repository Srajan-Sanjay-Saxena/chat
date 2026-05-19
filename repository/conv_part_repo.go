package repository

import (
	"context"
	"github.com/google/uuid"
)

func (r *Repository) AddParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {

	// Writing SQL query to insert a new participant into the conversation_participants table
	query := `insert into conversation_participants (conversation_id, user_id) values ($1, $2)`

	// Executing the query
	_, err := r.DB.Exec(ctx, query, conversationID, userID)
	return err
}

func (r *Repository) RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {

	// Writing SQL query to delete a participant from the conversation_participants table
	query := `delete from conversation_participants where conversation_id = $1 and user_id = $2`

	// Executing the query
	_, err := r.DB.Exec(ctx, query, conversationID, userID)
	return err
}

func (r *Repository) GetParticipantsByConversationID(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {

	// Writing SQL query to select user ids of participants in a conversation
	query := `select user_id from conversation_participants where conversation_id = $1`

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