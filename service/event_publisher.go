package service

import (
	// "chat-v2/db"
	"context"
)

type EventBus interface {
	Publish(ctx context.Context, msg *OutMessage) error
	Subscribe(ctx context.Context, handler MessageHandler) error
}

type MessageHandler func(*OutMessage)

