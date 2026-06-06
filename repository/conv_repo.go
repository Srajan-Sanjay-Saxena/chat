package repository

import (
	"chat-v2/db"
	"context"
	"errors"
	"time"
	"chat-v2/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrConversationExists = errors.New("conversation already exists")

func (r *Repository) CreateConversation(ctx context.Context, conversation *db.Conversation) error {
	// Insert conversation including type and display_name
	if conversation.Type == "" {
		conversation.Type = "group"
	}
	query := `insert into conversations (id, type, title, display_name, canonical_name, created_at) values ($1, $2, $3, $4, $5, $6) returning id, type, title, display_name, canonical_name, created_at`

	return r.DB.QueryRow(ctx, query, conversation.ID, conversation.Type, conversation.Title, conversation.DisplayName, conversation.CanonicalName, conversation.CreatedAt).
		Scan(&conversation.ID, &conversation.Type, &conversation.Title, &conversation.DisplayName, &conversation.CanonicalName, &conversation.CreatedAt)
}

func (r *Repository) GetConversationByID(ctx context.Context, id uuid.UUID) (*db.Conversation, error) {
	query := `select id, type, title, display_name, canonical_name, created_at from conversations where id = $1`

	var conversation db.Conversation
	err := r.DB.QueryRow(ctx, query, id).Scan(&conversation.ID, &conversation.Type, &conversation.Title, &conversation.DisplayName, &conversation.CanonicalName, &conversation.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (r *Repository) GetConversationsByUserID(ctx context.Context, userID uuid.UUID) ([]*db.Conversation, error) {
	query := `select c.id, c.type, c.title, c.display_name, c.canonical_name, c.created_at from conversations c 
			  join conversation_participants cp on c.id = cp.conversation_id 
			  where cp.user_id = $1 order by c.created_at`

	rows, err := r.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []*db.Conversation
	for rows.Next() {
		var conversation db.Conversation
		if err := rows.Scan(&conversation.ID, &conversation.Type, &conversation.Title, &conversation.DisplayName, &conversation.CanonicalName, &conversation.CreatedAt); err != nil {
			return nil, err
		}
		conversations = append(conversations, &conversation)
	}
	return conversations, nil
}

// GetConversationsWithOtherUsernameByUserID returns conversations for a user
// and, for private conversations, the other participant's username as the
// returned Conversation.DisplayName value (server-computed).
func (r *Repository) GetConversationsWithOtherUsernameByUserID(ctx context.Context, userID uuid.UUID) ([]*db.Conversation, error) {
	query := `
		select c.id, c.type, c.title, c.display_name, c.canonical_name, c.created_at, u.username
		from conversations c
		join conversation_participants cp on c.id = cp.conversation_id and cp.user_id = $1
		left join conversation_participants cp_other on cp_other.conversation_id = c.id and cp_other.user_id <> $1 and c.type = 'private'
		left join users u on u.id = cp_other.user_id
		order by c.created_at
	`

	rows, err := r.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []*db.Conversation
	for rows.Next() {
		var conv db.Conversation
		var otherUsername *string
		if err := rows.Scan(&conv.ID, &conv.Type, &conv.Title, &conv.DisplayName, &conv.CanonicalName, &conv.CreatedAt, &otherUsername); err != nil {
			return nil, err
		}
		// If it's a private conversation, prefer the other participant's username.
		if conv.Type == "private" && otherUsername != nil && *otherUsername != "" {
			conv.DisplayName = *otherUsername
		}
		conversations = append(conversations, &conv)
	}
	return conversations, nil
}

func (r *Repository) CreateConversationWithParticipants(ctx context.Context, conversation *db.Conversation, participantIDs []uuid.UUID) error {
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
	// ensure conversation has ID and CreatedAt
	if conversation.ID == uuid.Nil {
		conversation.ID = uuid.New()
	}
	if conversation.CreatedAt.IsZero() {
		conversation.CreatedAt = time.Now()
	}
	if conversation.Type == "" {
		conversation.Type = "group"
	}

	query := `
		insert into conversations (id, type, title, display_name, canonical_name, created_at)
		values ($1, $2, $3, $4, $5, $6)
		returning id
	`
	err = tx.QueryRow(ctx, query, conversation.ID, conversation.Type, conversation.Title, conversation.DisplayName, conversation.CanonicalName, conversation.CreatedAt).Scan(&conversation.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Unique violation - conversation with same canonical_name likely exists
			return ErrConversationExists
		}
		return err
	}

	// Insert participants
	for _, userID := range participantIDs {
		_, err := tx.Exec(ctx, `insert into conversation_participants (conversation_id, user_id) values ($1, $2)`, conversation.ID, userID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) CreateConversationWithParticipantsByUsernames( ctx context.Context, conversation *db.Conversation, participantUsernames []string,) (err error) {

	logger.Log.Debug("Creating conversation with participants by usernames", "type", conversation.Type, "conversation_title", conversation.Title, "participant_usernames", participantUsernames)

    tx, err := r.DB.Begin(ctx)
    if err != nil {
        return err
    }

    defer func() {
        if err != nil {
            _ = tx.Rollback(ctx)
        } else {
            _ = tx.Commit(ctx)
        }
    }()

    uniqueUsernames := uniqueStrings(participantUsernames)

    // ---- Validate usernames exist ----

	var missingCount int

	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM unnest($1::text[]) input(username)
		LEFT JOIN users u
			ON u.username = input.username
		WHERE u.id IS NULL
	`, uniqueUsernames).Scan(&missingCount)

	if err != nil {
		return err
	}

	if missingCount > 0 {
		return errors.New("one or more usernames were not found")
	}

    // ---- Create conversation ----

    if conversation.ID == uuid.Nil {
        conversation.ID = uuid.New()
    }

    if conversation.CreatedAt.IsZero() {
        conversation.CreatedAt = time.Now()
    }

    if conversation.Type == "" {
        conversation.Type = "group"
    }

    err = tx.QueryRow(ctx, `
        INSERT INTO conversations
            (id, type, title, display_name, canonical_name, created_at)
        VALUES
            ($1,$2,$3,$4,$5,$6)
        RETURNING id
    `,
        conversation.ID,
        conversation.Type,
        conversation.Title,
        conversation.DisplayName,
        conversation.CanonicalName,
        conversation.CreatedAt,
    ).Scan(&conversation.ID)

    if err != nil {
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "23505" {
            return ErrConversationExists
        }
        return err
    }

    // ---- Insert participants ----

    _, err = tx.Exec(ctx, `
        INSERT INTO conversation_participants (conversation_id, user_id)
        SELECT $1, id
        FROM users
        WHERE username = ANY($2::text[])
    `, conversation.ID, uniqueUsernames)

    if err != nil {
        return err
    }

    return nil
}

func (r *Repository) GetConversationParticipants(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	query := `select user_id from conversation_participants where conversation_id = $1`

	rows, err := r.DB.Query(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participantIDs []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		participantIDs = append(participantIDs, userID)
	}
	return participantIDs, nil
}

func (r *Repository) AddConversationParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	_, err := r.DB.Exec(ctx, `insert into conversation_participants (conversation_id, user_id) values ($1, $2)`, conversationID, userID)
	return err
}

func (r *Repository) RemoveConversationParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	_, err := r.DB.Exec(ctx, `delete from conversation_participants where conversation_id = $1 and user_id = $2`, conversationID, userID)
	return err
}

func (r *Repository) GetConversationByTitle(ctx context.Context, title string) (*db.Conversation, error) {
	query := `select id, type, title, display_name, canonical_name, created_at from conversations where title = $1`

	var conversation db.Conversation
	err := r.DB.QueryRow(ctx, query, title).Scan(&conversation.ID, &conversation.Type, &conversation.Title, &conversation.DisplayName, &conversation.CanonicalName, &conversation.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (r *Repository) GetConversationByCanonicalName(ctx context.Context, canonical string) (*db.Conversation, error) {
	query := `select id, type, title, display_name, canonical_name, created_at from conversations where canonical_name = $1 and type = 'private'`

	var conversation db.Conversation
	err := r.DB.QueryRow(ctx, query, canonical).Scan(&conversation.ID, &conversation.Type, &conversation.Title, &conversation.DisplayName, &conversation.CanonicalName, &conversation.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}
