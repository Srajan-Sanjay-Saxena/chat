package conversation

import (
	"chat-v2/internal/domain/ent"
	"chat-v2/internal/domain/ent/conversation"
	"chat-v2/internal/domain/ent/conversationparticipant"
	"chat-v2/internal/domain/ent/user"
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrConversationExists = errors.New("conversation already exists")
var ErrUsersNotFound = errors.New("one or more usernames not found")

type Repository struct {
	client *ent.Client
}

func NewRepository(client *ent.Client) *Repository {
	return &Repository{client: client}
}

func (r *Repository) Create(ctx context.Context, convType, title, displayName, canonicalName string) (*ent.Conversation, error) {
	builder := r.client.Conversation.Create().
		SetType(conversation.Type(convType))

	if title != "" {
		builder.SetTitle(title)
	}
	if displayName != "" {
		builder.SetDisplayName(displayName)
	}
	if canonicalName != "" {
		builder.SetCanonicalName(canonicalName)
	}

	conv, err := builder.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrConversationExists
		}
		return nil, err
	}
	return conv, nil
}

func (r *Repository) CreateWithParticipants(ctx context.Context, convType, title, displayName, canonicalName string, usernames []string) (*ent.Conversation, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch users
	users, err := tx.User.Query().
		Where(user.UsernameIn(usernames...)).
		All(ctx)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if len(users) != len(usernames) {
		tx.Rollback()
		return nil, ErrUsersNotFound
	}

	// Create conversation
	builder := tx.Conversation.Create().
		SetType(conversation.Type(convType))

	if title != "" {
		builder.SetTitle(title)
	}
	if displayName != "" {
		builder.SetDisplayName(displayName)
	}
	if canonicalName != "" {
		builder.SetCanonicalName(canonicalName)
	}

	conv, err := builder.Save(ctx)
	if err != nil {
		tx.Rollback()
		if ent.IsConstraintError(err) {
			return nil, ErrConversationExists
		}
		return nil, err
	}

	// Add participants
	for _, u := range users {
		_, err := tx.ConversationParticipant.Create().
			SetConversationID(conv.ID).
			SetUserID(u.ID).
			Save(ctx)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return conv, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*ent.Conversation, error) {
	return r.client.Conversation.Get(ctx, id)
}

func (r *Repository) GetByCanonicalName(ctx context.Context, canonicalName string) (*ent.Conversation, error) {
	return r.client.Conversation.Query().
		Where(
			conversation.CanonicalNameEQ(canonicalName),
			conversation.TypeEQ(conversation.TypePrivate),
		).
		Only(ctx)
}

func (r *Repository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*ent.Conversation, error) {
	return r.client.Conversation.Query().
		Where(conversation.HasParticipantsWith(conversationparticipant.UserIDEQ(userID))).
		Order(ent.Asc(conversation.FieldCreatedAt)).
		All(ctx)
}

type ConversationWithDisplay struct {
	*ent.Conversation
	DisplayName *string `json:"display_name,omitempty"`
}

func (r *Repository) GetByUserIDWithDisplay(ctx context.Context, userID uuid.UUID) ([]*ConversationWithDisplay, error) {
	convs, err := r.client.Conversation.Query().
		Where(conversation.HasParticipantsWith(conversationparticipant.UserIDEQ(userID))).
		WithParticipants(func(q *ent.ConversationParticipantQuery) {
			q.WithUser()
		}).
		Order(ent.Asc(conversation.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*ConversationWithDisplay, 0, len(convs))
	for _, conv := range convs {
		cwd := &ConversationWithDisplay{
			Conversation: conv,
			DisplayName:  conv.DisplayName,
		}

		if conv.Type == conversation.TypePrivate {
			for _, p := range conv.Edges.Participants {
				if p.UserID != userID && p.Edges.User != nil {
					cwd.DisplayName = &p.Edges.User.Username
					break
				}
			}
		}

		result = append(result, cwd)
	}

	return result, nil
}

func (r *Repository) AddParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	_, err := r.client.ConversationParticipant.Create().
		SetConversationID(conversationID).
		SetUserID(userID).
		Save(ctx)
	return err
}

func (r *Repository) RemoveParticipant(ctx context.Context, conversationID, userID uuid.UUID) error {
	_, err := r.client.ConversationParticipant.Delete().
		Where(
			conversationparticipant.ConversationIDEQ(conversationID),
			conversationparticipant.UserIDEQ(userID),
		).
		Exec(ctx)
	return err
}

func (r *Repository) GetParticipants(ctx context.Context, conversationID uuid.UUID) ([]uuid.UUID, error) {
	participants, err := r.client.ConversationParticipant.Query().
		Where(conversationparticipant.ConversationIDEQ(conversationID)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, len(participants))
	for i, p := range participants {
		ids[i] = p.UserID
	}
	return ids, nil
}

func (r *Repository) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	return r.client.ConversationParticipant.Query().
		Where(
			conversationparticipant.ConversationIDEQ(conversationID),
			conversationparticipant.UserIDEQ(userID),
		).
		Exist(ctx)
}

func (r *Repository) GetFriends(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	convs, err := r.client.Conversation.Query().
		Where(
			conversation.TypeEQ(conversation.TypePrivate),
			conversation.HasParticipantsWith(conversationparticipant.UserIDEQ(userID)),
		).
		WithParticipants().
		All(ctx)
	if err != nil {
		return nil, err
	}

	var friendIDs []uuid.UUID
	for _, conv := range convs {
		for _, p := range conv.Edges.Participants {
			if p.UserID != userID {
				friendIDs = append(friendIDs, p.UserID)
			}
		}
	}

	return friendIDs, nil
}
