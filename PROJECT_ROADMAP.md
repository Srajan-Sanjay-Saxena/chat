# Real-Time Chat System — Project Roadmap

Target: A polished, deployable backend that is interview-explainable, technically solid, realistically finishable, and extensible.

Stack: Go, PostgreSQL, Redis, WebSockets, JWT, Docker, Minimal React frontend

---

## Phase 0 — Foundation

Goal: Clean repo + architecture

### Task 0.1 — Create Repository
- Repo name: `realtime-chat-system`

### Task 0.2 — Setup Folder Structure
```
/cmd/server
/internal
    /auth
    /chat
    /ws
    /db
    /models
    /middleware
    /redis
    /config
    /utils
/docker
```

### Task 0.3 — Initialize Go
- Subtasks:
  - `go mod init`
  - setup env loading
  - setup logger
  - create config struct
- Deliverable: Server starts successfully

---

## Phase 1 — Database Layer

Goal: Stable persistence model

### Task 1.1 — PostgreSQL Setup
- Subtasks:
  - install postgres
  - create DB
  - connect from Go
  - test ping
- Deliverable: Successful DB connection

### Task 1.2 — Design Schema
Tables:
- users
- conversations
- conversation_participants
- messages

Optional later:
- message_reads
- attachments

### Task 1.3 — Write SQL Migrations
- Subtasks:
  - create schema files
  - indexes
  - constraints
  - timestamps
- Important indexes:
  - messages(conversation_id, created_at)
  - conversation_participants(user_id)

### Task 1.4 — Repository Layer
- Subtasks:
  - create user repository
  - create message repository
  - create conversation repository
- Goal: DB logic separated from handlers

---

## Phase 2 — Authentication

Goal: Secure user system

### Task 2.1 — Signup API
- Subtasks:
  - validate request
  - hash password using bcrypt
  - insert user
  - handle duplicate username
- Endpoint: `POST /signup`

### Task 2.2 — Login API
- Subtasks:
  - verify password
  - generate JWT
  - return token
- Endpoint: `POST /login`

### Task 2.3 — JWT Middleware
- Subtasks:
  - parse bearer token
  - validate signature
  - extract user id
  - inject into request context
- Deliverable: Protected endpoints work

---

## Phase 3 — WebSocket Core

**This is the most important phase.**

Goal: Real-time communication architecture

### Task 3.1 — WebSocket Upgrade Handler
- Subtasks:
  - upgrade HTTP connection
  - validate JWT before upgrade
  - map connection to user
- Endpoint: `/ws`

### Task 3.2 — Client Struct
- Fields:
  - Conn *websocket.Conn
  - Send chan []byte
  - UserID int

### Task 3.3 — Hub Struct
- Responsibilities:
  - active clients
  - register/unregister
  - broadcast routing

### Task 3.4 — Read Pump
- Responsibilities:
  - read incoming messages
  - validate payload
  - push into processing pipeline

### Task 3.5 — Write Pump
- Responsibilities:
  - write outgoing messages
  - ping/pong heartbeats
  - disconnect cleanup
- **Critical for interview discussion**

---

## Phase 4 — Messaging System

Goal: Real chat functionality

### Task 4.1 — Send Message Flow
Flow:
```
Client
 -> WebSocket
 -> Hub
 -> Redis Publish
 -> Receiver Push
 -> Async DB Save
```

### Task 4.2 — Private Conversations
- Subtasks:
  - create conversation
  - add participants
  - fetch conversation list

### Task 4.3 — Group Chat
- Subtasks:
  - create group
  - join/leave group
  - group message broadcast

### Task 4.4 — Message Persistence
- Subtasks:
  - async inserts
  - retry logic
  - timestamps

---

## Phase 5 — Redis Integration

Goal: Scalability + presence

### Task 5.1 — Redis Connection
- Subtasks:
  - connect
  - health check
  - graceful reconnect

### Task 5.2 — Pub/Sub
- Channels:
  - chat_messages
  - presence_updates
- Subtasks:
  - publish message events
  - subscribe listeners
  - route to local clients

### Task 5.3 — Presence System
- Logic:
  - heartbeat every 30s
  - Redis TTL key
  - online/offline updates

---

## Phase 6 — Pagination + Optimization

Goal: Production-like behavior

### Task 6.1 — Message Pagination
- Endpoint: `GET /messages?conversation_id=1&before=200&limit=50`

### Task 6.2 — Cursor-Based Loading
- Avoid: OFFSET
- Use: WHERE id < ?
- Interviewers like this

### Task 6.3 — Connection Cleanup
- Subtasks:
  - detect dead sockets
  - remove stale users
  - close goroutines safely

---

## Phase 7 — Frontend (Minimal)

Goal: Enough UI for demo. Do NOT overspend time here.

### Task 7.1 — Login Page
- Simple

### Task 7.2 — Chat Window
- Features:
  - send message
  - show history
  - online status

### Task 7.3 — Group Chat UI
- Minimal functionality only

---

## Phase 8 — Docker + Deployment

Goal: Professional presentation

### Task 8.1 — Dockerize Backend
- Files:
  - Dockerfile
  - docker-compose.yml
- Services:
  - app
  - postgres
  - redis

### Task 8.2 — Environment Variables
- Use: .env

### Task 8.3 — Deploy
- Good enough:
  - Render
  - Railway
  - DigitalOcean

---

## Phase 9 — Resume Polish

Very important.

### Task 9.1 — README
- Include:
  - architecture diagram
  - features
  - tech stack
  - scaling discussion
  - screenshots
  - API endpoints

### Task 9.2 — Architecture Diagram
- Need:
  ```
  Frontend
     |
  Go Backend
     |
  Redis Pub/Sub
     |
  PostgreSQL
  ```

---

## Priority Order

### Must Finish
1. JWT auth
2. WebSocket pumps
3. Redis Pub/Sub
4. Message persistence
5. Pagination
6. Docker

### Nice to Have
- read receipts
- typing indicators
- file uploads
- notifications

---

## What NOT to do now

Avoid:
- Kubernetes
- Kafka
- custom Redis
- microservices
- fancy frontend
- event sourcing
- CQRS

These are distraction traps for your current target.

---

Reference file: PROJECT_TODO.md (tracks current progress)
