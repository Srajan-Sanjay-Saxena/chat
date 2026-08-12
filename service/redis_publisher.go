package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
)

type RedisBus struct {
	RedisClient *redis.Client
}

func NewRedisBus(redisClient *redis.Client) *RedisBus {
	return &RedisBus{RedisClient: redisClient}
}

func (b *RedisBus) Publish(ctx context.Context, msg *OutMessage) error {
	if b == nil || b.RedisClient == nil {
		return nil
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	channel := "relay:conversation:" + msg.ConversationID.String()
	return b.RedisClient.Publish(ctx, channel, payload).Err()
}

func (b *RedisBus) Subscribe(ctx context.Context, handler MessageHandler) error {
	if b == nil || b.RedisClient == nil {
		return nil
	}

	sub := b.RedisClient.PSubscribe(ctx, "relay:conversation:*")

	go func() {
		defer sub.Close()
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case redisMsg, ok := <-ch:
				if !ok {
					return
				}
				var msg OutMessage
				if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
					continue
				}
				handler(&msg)
			}
		}
	}()

	return nil
}