# Relay – Real-Time Chat Backend

A high-performance real-time messaging backend built in Go, featuring WebSocket bi-directional communication, JWT authentication, Redis-backed presence tracking, and a distributed Pub/Sub event bus for horizontal scalability.

## Architecture

```
┌─────────────────────────────────────────┐
│         Client (REST / WebSocket)       │
└────────────────────┬────────────────────┘
                     │
┌────────────────────▼────────────────────┐
│   Handler Layer (handler/, realtime/)   │
│   JWT Middleware · CORS · Rate Limiter  │
└────────────────────┬────────────────────┘
                     │
┌────────────────────▼────────────────────┐
│          Service Layer (service/)       │
│   MessageService · SubscriptionService  │
│   CachedMessageService · EventBus      │
└───────┬────────────┬────────────┬───────┘
        │            │            │
        ▼            ▼            ▼
┌─────────────┐ ┌─────────┐ ┌──────────────┐
│  Repository │ │ EventBus│ │Presence Store│
│  (pgx/v5)  │ │Redis/Mem│ │  (Redis TTL) │
└──────┬──────┘ └────┬────┘ └──────┬───────┘
       ▼              ▼             ▼
  PostgreSQL     Redis Pub/Sub   Redis KV
```

## Features

- **Real-time messaging** – WebSocket connections with per-client `readPump`/`writePump` goroutines, ping/pong heartbeats, and idle connection sweeping
- **Distributed event fanout** – `RedisBus` Pub/Sub broadcasts messages across multiple server instances; falls back to in-memory `LocalBus` if Redis is unavailable
- **Presence tracking** – Redis key-value store with TTL heartbeats for online/offline status
- **Multi-level caching** – Redis cache with in-memory fallback for message history
- **Cursor-based pagination** – Composite index `(conversation_id, created_at DESC, id DESC)` for efficient message history queries at scale
- **Rate limiting** – Sliding-window rate limiter (Redis-backed or in-memory) with per-endpoint policies
- **JWT authentication** – HMAC-signed tokens via cookies/Bearer header with password entropy enforcement (bcrypt + go-password-validator)
- **Graceful shutdown** – Signal handling (`SIGINT`/`SIGTERM`) with connection draining and clean hub teardown
- **Docker deployment** – Multi-stage build with embedded Redis server via entrypoint script

## Tech Stack

| Component | Technology |
| :--- | :--- |
| Language | Go 1.25 |
| HTTP Server | `net/http` (stdlib) |
| WebSocket | `gorilla/websocket` |
| Database | PostgreSQL (`jackc/pgx/v5` with connection pooling) |
| Cache & Presence | Redis (`redis/go-redis/v9`) |
| Authentication | `golang-jwt/v5`, `bcrypt`, `go-password-validator` |
| Logging | `slog` (structured) |
| Containerization | Docker (multi-stage Alpine) |

## Database Schema

Four core tables with UUID primary keys:

- **users** – id, username, email, password_hash, created_at
- **conversations** – id, type (direct/group), title, created_at
- **conversation_participants** – composite PK (conversation_id, user_id), joined_at
- **messages** – id, conversation_id, sender_id, content, created_at

Indexes optimized for membership verification and cursor pagination.

## API Endpoints

| Method | Path | Description |
| :--- | :--- | :--- |
| GET | `/health` | Health check |
| POST | `/api/signup` | User registration |
| POST | `/api/login` | User login (returns JWT) |
| POST | `/api/logout` | User logout |
| GET | `/api/me` | Current user info |
| POST | `/api/conversation/create` | Create conversation |
| GET | `/api/conversation/list` | List user's conversations |
| POST | `/api/conversation/join` | Join a conversation |
| POST | `/api/conversation/leave` | Leave a conversation |
| GET | `/api/conversation/members` | List conversation members |
| GET | `/api/conversation/messages` | Paginated message history |
| GET | `/api/users/search` | Search users |
| GET | `/api/ws` | WebSocket connection (upgrade) |
| GET | `/api/presence` | User presence status |

## Getting Started

### Prerequisites

- Go 1.25+
- PostgreSQL
- Redis (optional – app gracefully degrades without it)

### Environment Variables

Create a `.env` file in the project root:

```env
PORT=8080
ENV=development
DB_SOURCE=postgres://user:pass@localhost:5432/chat_db?sslmode=disable
JWT_SECRET=your_secret_key
REDIS_ADDR=localhost:6379
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_DB=0
WS_ALLOWED_ORIGINS=http://localhost:3000
```

### Run Locally

```bash
go mod download
go run main.go
```

The server starts on the configured port (default `8080`). Schema migrations run automatically on startup.

### Run with Docker

The Docker image bundles an embedded Redis server. The `entrypoint.sh` starts Redis as a daemon before launching the Go application.

```bash
docker build -t relay-backend .
docker run -p 8080:8080 --env-file .env relay-backend
```

Or using Docker Compose:

```bash
docker compose up --build
```

## Running Tests

```bash
# All tests
go test ./...

# WebSocket & realtime integration tests
go test -v ./internal/realtime/

# Redis presence store tests
go test -v ./db/redis/

# End-to-end tests
go test -v -run TestE2E
```

## Project Structure

```
├── main.go                  # Entry point, wiring, graceful shutdown
├── config/                  # Environment configuration
├── db/                      # Database connection & migrations
│   └── redis/               # Redis connection & presence store
├── handler/                 # HTTP route handlers (auth, conversation, presence)
├── internal/
│   ├── cache/               # Cache interface (Redis + memory implementations)
│   └── realtime/            # WebSocket hub, pumps, connection limiter
├── middleware/              # JWT auth, CORS, rate limiting
├── repository/              # PostgreSQL data access layer
├── service/                 # Business logic (message, subscription, event bus)
├── helper/                  # JWT maker, password hashing, migrations
├── logger/                  # Structured logging (slog)
├── Dockerfile               # Multi-stage build with embedded Redis
├── docker-compose.yml       # Container orchestration
└── docs/                    # Architecture docs, plans, review guides
```

## Design Decisions

- **Fail-open Redis strategy** – If Redis is unreachable at boot, the app continues with in-memory alternatives (`LocalBus`, `MemoryCache`), ensuring single-node operability without external dependencies.
- **Two-goroutine-per-connection model** – Isolates read and write paths, enabling non-blocking broadcasts and clean connection lifecycle management.
- **Cursor pagination over offset** – Maintains constant query performance regardless of message volume (composite index seek vs. sequential scan).
- **Decoupled EventBus interface** – Swappable transport layer allows scaling from single-process to multi-instance without changing application logic.

## License

This project is for educational and portfolio purposes.
