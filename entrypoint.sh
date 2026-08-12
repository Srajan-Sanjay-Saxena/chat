#!/bin/sh
set -e

# Start embedded Redis server in background
echo "Starting local Redis server..."
redis-server --daemonize yes --protected-mode no

# Wait for Redis server to be ready
for i in $(seq 1 10); do
  if redis-cli ping > /dev/null 2>&1; then
    echo "Redis server is ready!"
    break
  fi
  echo "Waiting for Redis server to start..."
  sleep 1
done

# Start the Go server
echo "Starting Chat app server..."
exec ./server
