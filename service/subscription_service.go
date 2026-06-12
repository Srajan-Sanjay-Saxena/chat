package service

import (
	"chat-v2/repository"
	"context"
	"github.com/google/uuid"
)

type SubscriptionService struct {
	repo *repository.Repository
}

func NewSubscriptionService(repo *repository.Repository) *SubscriptionService {
	return &SubscriptionService{repo: repo}
}

func (s *SubscriptionService) IsParticipant(ctx context.Context, conversationID, userID uuid.UUID) (bool, error) {
	return s.repo.IsParticipant(ctx, conversationID, userID)
}
