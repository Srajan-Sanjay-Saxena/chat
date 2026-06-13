package service

import (
	// "chat-v2/db"
	"context"
)

type EventPublisher interface {
	PublishMessage(ctx context.Context, msg *OutMessage) error
}
