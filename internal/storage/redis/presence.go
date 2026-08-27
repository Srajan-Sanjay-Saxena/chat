package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

type PresenceStore struct {
	client *goredis.Client
}

type Presence struct {
	Online   bool
	LastSeen time.Time
}

type PresenceInfo struct {
	OnlineMembers []uuid.UUID
	TotalMembers  int
}

func NewPresenceStore(client *goredis.Client) *PresenceStore {
	return &PresenceStore{client: client}
}

func (p *PresenceStore) Update(ctx context.Context, userID uuid.UUID) error {
	if p == nil || p.client == nil {
		return nil
	}
	key := fmt.Sprintf("presence:%s", userID)
	return p.client.Set(ctx, key, time.Now().Unix(), 2*time.Minute).Err()
}

func (p *PresenceStore) Get(ctx context.Context, userID uuid.UUID) (Presence, error) {
	if p == nil || p.client == nil {
		return Presence{Online: false}, nil
	}

	key := fmt.Sprintf("presence:%s", userID)
	val, err := p.client.Get(ctx, key).Int64()
	if err != nil {
		if err == goredis.Nil {
			return Presence{Online: false}, nil
		}
		return Presence{}, err
	}

	return Presence{Online: true, LastSeen: time.Unix(val, 0)}, nil
}

func (p *PresenceStore) GetBulk(ctx context.Context, userIDs []uuid.UUID) (PresenceInfo, error) {
	if p == nil || p.client == nil || len(userIDs) == 0 {
		return PresenceInfo{}, nil
	}

	keys := make([]string, len(userIDs))
	for i, uid := range userIDs {
		keys[i] = fmt.Sprintf("presence:%s", uid.String())
	}

	results, err := p.client.MGet(ctx, keys...).Result()
	if err != nil {
		return PresenceInfo{}, fmt.Errorf("MGET failed: %w", err)
	}

	var onlineMembers []uuid.UUID
	for i, res := range results {
		if res != nil {
			onlineMembers = append(onlineMembers, userIDs[i])
		}
	}

	return PresenceInfo{
		OnlineMembers: onlineMembers,
		TotalMembers:  len(userIDs),
	}, nil
}
