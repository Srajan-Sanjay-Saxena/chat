package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const devContainerName = "relay-redis-dev"

// NewClient creates a new Redis client and checks the connection.
func NewClient(addr, username, password string, db int, useTLS bool) (*goredis.Client, error) {
	// Create a new Redis client
	opt := &goredis.Options{
		Addr:        addr,
		Username:    username,
		Password:    password,
		DB:          db,
		DialTimeout: 10 * time.Second,
	}

	if useTLS {
		opt.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := goredis.NewClient(
		opt,
	)

	// Ping the Redis server to check the connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return client, nil
}
