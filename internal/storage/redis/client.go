package redis

import (
	"chat-v2/internal/pkg/logger"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const devContainerName = "relay-redis-dev"

func NewClient(addr, username, password string, db int) (*goredis.Client, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, nil
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:        addr,
		Password:    password,
		DB:          db,
		DialTimeout: 2 * time.Second,
		MaxRetries:  1,
		PoolSize:    1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}

	// Reconnect with full pool
	client.Close()
	client = goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return client, nil
}

func NewClientOrBoot(addr, username, password string, db int, env string) (*goredis.Client, error) {
	client, err := NewClient(addr, username, password, db)
	if err == nil && client != nil {
		return client, nil
	}

	if env != "development" {
		return nil, err
	}

	if !isDockerAvailable() {
		return nil, nil
	}

	if strings.TrimSpace(addr) == "" {
		addr = "127.0.0.1:6379"
	}

	if isContainerExists(devContainerName) {
		if !isContainerRunning(devContainerName) {
			if err := startContainer(devContainerName); err != nil {
				return nil, nil
			}
		}
	} else {
		if err := runRedisContainer(addr); err != nil {
			return nil, nil
		}
	}

	return waitAndConnect(addr, username, password, db)
}

func isDockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

func runRedisContainer(addr string) error {
	port := extractPort(addr)
	cmd := exec.Command("docker", "run", "-d",
		"--name", devContainerName,
		"-p", fmt.Sprintf("%s:6379", port),
		"redis:7-alpine",
	)
	return cmd.Run()
}

func isContainerExists(name string) bool {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", name)
	return cmd.Run() == nil
}

func isContainerRunning(name string) bool {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", name)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

func startContainer(name string) error {
	cmd := exec.Command("docker", "start", name)
	return cmd.Run()
}

func waitAndConnect(addr, username, password string, db int) (*goredis.Client, error) {
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)

		client := goredis.NewClient(&goredis.Options{
			Addr:     addr,
			Username: username,
			Password: password,
			DB:       db,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := client.Ping(ctx).Err()
		cancel()

		if err == nil {
			logger.Info("Redis connected after bootstrap", "addr", addr)
			return client, nil
		}
		client.Close()
	}

	return nil, fmt.Errorf("redis not reachable after 10 attempts")
}

func extractPort(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[i+1:]
		}
	}
	return "6379"
}
