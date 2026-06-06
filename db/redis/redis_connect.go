package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"chat-v2/logger"
	"fmt"
	"time"
	"github.com/google/uuid"
)

type PresenceStore struct {
	redisClient *redis.Client
}

type Presence struct {
	Online bool
	LastSeen time.Time
}

type PresenceInfo struct {
	OnlineMembers int
	OfflineMembers int
	TotalMembers int
}

func Connect(addr string, Username string, password string, db int) (*redis.Client, error) {
	RedisClient := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		logger.Log.Error("Failed to connect to Redis", "error", err)
		return nil, err
	}
	logger.Log.Info(fmt.Sprintf("Connected to Redis at %s", addr))
	return RedisClient, nil
}

func NewPresenceStore(redisClient *redis.Client) *PresenceStore {
	return &PresenceStore{
		redisClient: redisClient,
	}
}

func (p *PresenceStore) Close() error {
	if p.redisClient != nil {
		return p.redisClient.Close()
	}
	return nil
}

func (p *PresenceStore) UpdatePresence(ctx context.Context, userID string) error {
	if p.redisClient == nil {
		return fmt.Errorf("Redis client is not initialized")
	}
	key := fmt.Sprintf("presence:%s", userID)
	return p.redisClient.Set(ctx, key, time.Now().Unix(), 2*time.Minute).Err()
}


func (p *PresenceStore) GetPresence(ctx context.Context, userID string) (Presence, error) {
	if p.redisClient == nil {
		return Presence{}, fmt.Errorf("Redis client is not initialized")
	}
	key := fmt.Sprintf("presence:%s", userID)
	val, err := p.redisClient.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return Presence{Online: false}, nil
		}
		return Presence{}, err
	}
	
	return Presence{Online: true, LastSeen: time.Unix(val, 0)}, nil
}

func (p *PresenceStore) GetConversationPresence(ctx context.Context, userIDs []uuid.UUID) (PresenceInfo, error) {
	if p.redisClient == nil {
		return PresenceInfo{}, fmt.Errorf("Redis client is not initialized")
	}
	
	totalMembers := len(userIDs)
	if totalMembers == 0 {
		return PresenceInfo{}, nil
	}

	onlineMembers := 0
	for _, userID := range userIDs {
		presence, err := p.GetPresence(ctx, userID.String())
		if err != nil {
			return PresenceInfo{}, err
		}
		if presence.Online {
			onlineMembers++
		}
	}

	return PresenceInfo{
		OnlineMembers: onlineMembers,
		OfflineMembers: totalMembers - onlineMembers,
		TotalMembers: totalMembers,
	}, nil
}

func (p *PresenceStore) GetMassPresence(ctx context.Context, userIDs []uuid.UUID) (map[string]Presence, error) {
	if p.redisClient == nil {
		return nil, fmt.Errorf("Redis client is not initialized")
	}

	presenceMap := make(map[string]Presence)
	for _, userID := range userIDs {
		presence, err := p.GetPresence(ctx, userID.String())
		if err != nil {
			return nil, err
		}
		presenceMap[userID.String()] = presence
	}
	return presenceMap, nil
}