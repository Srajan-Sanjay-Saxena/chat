package redis

import (
	"chat-v2/logger"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type PresenceStore struct {
	redisClient *redis.Client
}

type Presence struct {
	Online   bool
	LastSeen time.Time
}

type PresenceInfo struct {
	OnlineMembers []uuid.UUID
	TotalMembers  int
}

func Connect(addr string, Username string, password string, db int) (*redis.Client, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, nil
	}

	RedisClient := redis.NewClient(&redis.Options{
		Addr:        addr,
		Password:    password,
		DB:          db,
		DialTimeout: 2 * time.Second,
		MaxRetries:  1,
		PoolSize:    1, // minimal pool during initial probe
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		RedisClient.Close()
		logger.Log.Error("Failed to connect to Redis", "error", err)
		return nil, err
	}
	logger.Log.Info(fmt.Sprintf("Connected to Redis at %s", addr))

	// Reconnect with full pool settings now that we know Redis is reachable
	RedisClient.Close()
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
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

func (p *PresenceStore) UpdatePresence(ctx context.Context, userID uuid.UUID) error {
	if p == nil || p.redisClient == nil {
		return nil
	}
	key := fmt.Sprintf("presence:%s", userID)
	return p.redisClient.Set(ctx, key, time.Now().Unix(), 2*time.Minute).Err()
}

func (p *PresenceStore) GetPresence(ctx context.Context, userID uuid.UUID) (Presence, error) {
	if p == nil || p.redisClient == nil {
		return Presence{Online: false}, nil
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

func (p *PresenceStore) GetMassPresence(ctx context.Context, userIDs []uuid.UUID) (PresenceInfo, error) {
	if p == nil || p.redisClient == nil {
		return PresenceInfo{}, nil
	}

	totalMembers := len(userIDs)
	if totalMembers == 0 {
		return PresenceInfo{}, nil
	}

	var onlineMembers []uuid.UUID
	keys := make([]string, totalMembers)
	for i, uid := range userIDs {
		keys[i] = fmt.Sprintf("presence:%s", uid.String())
	}

	results, err := p.redisClient.MGet(ctx, keys...).Result()
	if err != nil {
		return PresenceInfo{}, fmt.Errorf("MGET failed: %w", err)
	}

	for i, res := range results {
		if res != nil {
			key := keys[i]
			userID := userIDs[i]

			onlineMembers = append(onlineMembers, userID)
			logger.Log.Debug(fmt.Sprintf("User %s is online (key: %s)", userID, key))
		}
	}

	return PresenceInfo{
		OnlineMembers: onlineMembers,
		TotalMembers:  totalMembers,
	}, nil
}
