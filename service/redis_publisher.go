package service

import(
	"github.com/redis/go-redis/v9"
	"context"
	"encoding/json"
)

type RedisBus struct {
	RedisClient *redis.Client
}

func NewRedisBus(redisClient *redis.Client) *RedisBus {
	return &RedisBus{RedisClient: redisClient}
}

func (b *RedisBus) Publish(ctx context.Context, msg *OutMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	channel := "relay:conversation:" + msg.ConversationID.String()
	err = b.RedisClient.Publish(ctx, channel, payload).Err()
	return err
}
// Publish using
// b.Publish(ctx, outMsg)

func (b *RedisBus) Subscribe(ctx context.Context, handler MessageHandler) error {

	sub := b.RedisClient.PSubscribe(ctx, "relay:conversation:*")

	go func() {
		for redisMsg := range sub.Channel() {
			var msg OutMessage
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				continue
			}

			handler(&msg)
		}
	}()

	return nil
}

// Subscribe using
// b.Subscribe(ctx, hub.Broadcast)