package redis

import (
	"chat-v2/logger"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const devContainerName = "relay-redis-dev"

// ConnectOrBoot tries to connect to Redis at the given addr.
// If the connection fails and the environment is "development", it attempts
// to spin up a Redis Docker container automatically. If Docker is unavailable
// or the container fails to start, it returns nil so the caller can fall back
// to in-memory alternatives.
func ConnectOrBoot(addr, username, password string, db int, env string) (*goredis.Client, error) {
	logger.Log.Info("Redis bootstrap: attempting connection", "addr", addr, "env", env)

	// 1. Try connecting directly
	client, err := Connect(addr, username, password, db)
	if err == nil && client != nil {
		logger.Log.Info("Redis bootstrap: connected successfully on first attempt", "addr", addr)
		return client, nil
	}

	if err != nil {
		logger.Log.Warn("Redis bootstrap: initial connection failed", "addr", addr, "error", err)
	} else {
		logger.Log.Info("Redis bootstrap: no address configured, skipping direct connect")
	}

	// 2. Only auto-boot in development
	if env != "development" {
		logger.Log.Warn("Redis bootstrap: not in development mode, skipping Docker auto-boot", "env", env)
		return nil, err
	}

	// 3. Check if Docker is available
	if !isDockerAvailable() {
		logger.Log.Warn("Redis bootstrap: Docker CLI not found or not running, cannot auto-boot Redis")
		return nil, nil
	}
	logger.Log.Info("Redis bootstrap: Docker CLI is available")

	// 4. Determine the target address (default to localhost:6379 if empty)
	if strings.TrimSpace(addr) == "" {
		addr = "127.0.0.1:6379"
		logger.Log.Info("Redis bootstrap: no REDIS_ADDR set, defaulting to", "addr", addr)
	}

	// 5. Check if container already exists
	if isContainerExists(devContainerName) {
		logger.Log.Info("Redis bootstrap: found existing container", "container", devContainerName)

		if isContainerRunning(devContainerName) {
			logger.Log.Info("Redis bootstrap: container is already running, retrying connection")
		} else {
			logger.Log.Info("Redis bootstrap: container exists but stopped, starting it...")
			if err := startContainer(devContainerName); err != nil {
				logger.Log.Error("Redis bootstrap: failed to start existing container", "container", devContainerName, "error", err)
				return nil, nil
			}
			logger.Log.Info("Redis bootstrap: existing container started successfully", "container", devContainerName)
		}
	} else {
		// 6. Run a new Redis container
		logger.Log.Info("Redis bootstrap: no existing container found, creating new one...", "container", devContainerName)
		if err := runRedisContainer(addr); err != nil {
			logger.Log.Error("Redis bootstrap: failed to create Redis container", "error", err)
			return nil, nil
		}
		logger.Log.Info("Redis bootstrap: new Redis container created successfully", "container", devContainerName)
	}

	// 7. Wait for Redis to become ready
	logger.Log.Info("Redis bootstrap: waiting for Redis to become ready...")
	client, err = waitAndConnect(addr, username, password, db)
	if err != nil {
		logger.Log.Error("Redis bootstrap: failed to connect after container start", "error", err)
		return nil, nil
	}
	if client == nil {
		logger.Log.Warn("Redis bootstrap: container started but Redis not reachable, falling back to in-memory")
		return nil, nil
	}

	logger.Log.Info("Redis bootstrap: successfully connected to auto-bootstrapped Redis", "addr", addr, "container", devContainerName)
	return client, nil
}

func isDockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Debug("Redis bootstrap: docker info check failed", "error", err, "output", string(output))
		return false
	}
	return true
}

func runRedisContainer(addr string) error {
	port := extractPort(addr)
	logger.Log.Info("Redis bootstrap: running docker container", "image", "redis:7-alpine", "port", port)

	cmd := exec.Command("docker", "run", "-d",
		"--name", devContainerName,
		"-p", fmt.Sprintf("%s:6379", port),
		"redis:7-alpine",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run failed: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	containerID := strings.TrimSpace(string(output))
	logger.Log.Info("Redis bootstrap: container created", "container_id", containerID[:12], "name", devContainerName)
	return nil
}

func isContainerExists(name string) bool {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", name)
	err := cmd.Run()
	return err == nil
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker start failed: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func waitAndConnect(addr, username, password string, db int) (*goredis.Client, error) {
	maxRetries := 10
	retryInterval := 500 * time.Millisecond

	for i := 1; i <= maxRetries; i++ {
		time.Sleep(retryInterval)
		logger.Log.Debug("Redis bootstrap: connection attempt", "attempt", i, "max", maxRetries)

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
			logger.Log.Info("Redis bootstrap: connection succeeded", "attempt", i, "addr", addr)
			return client, nil
		}

		logger.Log.Debug("Redis bootstrap: ping failed, retrying...", "attempt", i, "error", err)
		client.Close()
	}

	return nil, fmt.Errorf("redis not reachable after %d attempts (%v total wait)", maxRetries, time.Duration(maxRetries)*retryInterval)
}

func extractPort(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[i+1:]
		}
	}
	return "6379"
}
