# 💬 Chat Backend

A real-time chat application backend written in Go.

## ✨ Features

- WebSocket-based real-time messaging
- JWT authentication
- PostgreSQL for persistent storage
- Redis for presence tracking
- Clean architecture with repository/service pattern
- Background idle connection sweeper

## 🚀 Getting Started

### Prerequisites

- Go 1.25+
- PostgreSQL
- Redis

### Environment Variables

Create a `.env` file in the root directory with the following variables:

```env
PORT=
dbSource=
JWT_SECRET=
WSAllowedOrigins=
USE_DOCKER_TESTDB=0
dbSource2=
REDIS_ADDR=
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_DB=
```

### Run Locally

```bash
go mod download
go run main.go
```

### Run with Docker

The Docker image includes an embedded Redis server. When you start the container, `entrypoint.sh` starts the internal Redis server daemon and then launches the Go app server.

```bash
docker build -t chat-backend .
docker run -p 8080:8080 --env-file .env chat-backend
```

### 🧪 Running Tests

To run unit and integration tests (including Redis presence tests):

```bash
# Run all tests across packages
go test ./...

# Run Redis presence store tests specifically
go test -v ./db/redis
```

For detailed documentation on the internal Redis container setup and architecture, see [docs/redis_container_migration.md](file:///C:/Users/shash/projects/chat/docs/redis_container_migration.md).

## 🛠️ Tech Stack

- **Go** – backend logic
- **Gorilla WebSocket** – real-time communication
- **pgx** – PostgreSQL driver
- **go-redis** – Redis client
- **golang-jwt** – authentication
