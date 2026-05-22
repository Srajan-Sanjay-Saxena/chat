# Real-Time Chat System — Project TODO

Status: **In Progress**  
Current Phase: **3 (WebSocket core)**  
Last Updated: May 19, 2026

---

## Phase 0 — Foundation ✅ COMPLETE

- [x] Task 0.1 — Create Repository (realtime-chat-system)
- [x] Task 0.2 — Setup Folder Structure
  - [x] /cmd/server
  - [x] /internal (partial)
  - [x] /db
  - [x] /config
  - [x] /logger
  - [x] /helper
  - [x] /handler
  - [x] /repository
- [x] Task 0.3 — Initialize Go
  - [x] go mod init
  - [x] setup env loading (.env + godotenv)
  - [x] setup logger (slog)
  - [x] create config struct
  - [x] Server starts successfully

---

## Phase 1 — Database Layer ✅ COMPLETE

- [x] Task 1.1 — PostgreSQL Setup
  - [x] Install postgres (Neon)
  - [x] Create DB
  - [x] Connect from Go (pgxpool)
  - [x] Test ping
  - [x] Successful DB connection

- [x] Task 1.2 — Design Schema
  - [x] users table
  - [x] conversations table
  - [x] conversation_participants table
  - [x] messages table
  - [x] Fields normalized and minimal for phase-1

- [x] Task 1.3 — Write SQL Migrations
  - [x] Create schema files (0001_init.up.sql, down.sql)
  - [x] Add indexes (conversation_participants, messages)
  - [x] Add constraints (FK, UNIQUE, NOT NULL)
  - [x] Add timestamps (created_at)
  - [x] CREATE EXTENSION uuid-ossp

- [x] Task 1.4 — Repository Layer
  - [x] UserRepository (Create, GetByID, GetByUsername)
  - [x] MessageRepository (Create, GetByID, GetByConversationID)
  - [x] ConversationRepository (Create, GetByID, ListByUser)
  - [x] Error handling (ErrUserExists sentinel)
  - [x] DB logic separated from handlers

---

## Phase 2 — Authentication ✅ COMPLETE

- [x] Task 2.1 — Signup API
  - [x] Validate request (non-empty fields)
  - [x] Email format validation (helper.ValidateEmail)
  - [x] Password strength validation (helper.ValidatePassword)
  - [x] Username format validation (helper.ValidateUsername)
  - [x] Hash password using bcrypt (helper.HashPassword)
  - [x] Insert user via repository
  - [x] Handle duplicate username (409 Conflict + errors.Is)
  - [x] Return 201 Created with user_id
  - [x] POST /signup endpoint registered

- [x] Task 2.2 — Login API
  - [x] Verify password (helper.CheckPasswordHash)
  - [x] Generate JWT token (helper.JWTMaker.CreateToken)
  - [x] Return token in response JSON
  - [x] Include expires_at for client reference
  - [x] Return 200 OK on success
  - [x] Return 401 Unauthorized on invalid credentials
  - [x] POST /login endpoint registered

- [x] Task 2.3 — JWT Middleware
  - [x] Create middleware function
  - [x] Parse bearer token from Authorization header
  - [x] Validate JWT signature
  - [x] Handle expired tokens
  - [x] Extract user ID from claims
  - [x] Inject user ID into request context
  - [x] Test on protected endpoints

---

## Phase 3 — WebSocket Core 🔄 IN PROGRESS

- [ ] Task 3.1 — WebSocket Upgrade Handler
  - [ ] Upgrade HTTP to WebSocket
  - [ ] Validate JWT before upgrade
  - [ ] Map connection to user
  - [ ] Return user context or error
  - [ ] Endpoint: /ws

- [ ] Task 3.2 — Client Struct
  - [ ] Define Client struct (Conn, Send, UserID)
  - [ ] Client connection state
  - [ ] Client cleanup methods

- [ ] Task 3.3 — Hub Struct
  - [ ] Define Hub struct (clients map, register/unregister channels)
  - [ ] Active clients tracking
  - [ ] Register/unregister logic
  - [ ] Broadcast routing

- [ ] Task 3.4 — Read Pump
  - [ ] Read incoming messages from client
  - [ ] Parse JSON payload
  - [ ] Validate message structure
  - [ ] Push to processing pipeline
  - [ ] Error handling (disconnect on read error)

- [ ] Task 3.5 — Write Pump
  - [ ] Write outgoing messages to client
  - [ ] Implement ping/pong heartbeats
  - [ ] Handle graceful disconnect
  - [ ] Close goroutines safely
  - [ ] **Critical for interview discussion**

---

## Phase 4 — Messaging System ⏳ NOT STARTED

- [x] Task 4.1 — Send Message Flow
  - [x] Client → WebSocket → Hub → Conversation Subscribers → PostgreSQL Save
  - [x] Message struct definition
  - [x] Routing logic
  - [x] Error handling

- [x] Task 4.2 — Private Conversations
  - [x] Create conversation endpoint
  - [x] Add participants to conversation
  - [x] Fetch conversation list for user
  - [x] Validate permissions

- [ ] Task 4.3 — Group Chat
  - [ ] Create group endpoint
  - [ ] Join/leave group logic
  - [ ] Group message broadcast
  - [ ] Member list

- [ ] Task 4.4 — Message Persistence
  - [ ] Async message insert to DB
  - [ ] Retry logic for failed inserts
  - [ ] Timestamps accuracy
  - [ ] Idempotency (duplicate detection)

---

## Phase 5 — Redis Integration ⏳ NOT STARTED

- [ ] Task 5.1 — Post-MVP Scaling
  - [ ] Add Redis only after single-server chat works
  - [ ] Use Redis for multi-instance broadcast
  - [ ] Add Redis-based presence later
  - [ ] Keep this phase out of the 15-day MVP

---

## Phase 6 — Pagination + Optimization ⏳ NOT STARTED

- [ ] Task 6.1 — Message Pagination
  - [ ] Endpoint: GET /messages?conversation_id=X&before=Y&limit=50
  - [ ] Query builder with WHERE id < ?
  - [ ] Limit results
  - [ ] Return cursor for next page

- [ ] Task 6.2 — Cursor-Based Loading
  - [ ] Avoid OFFSET (use WHERE id < ?)
  - [ ] Efficient index usage
  - [ ] Test with large datasets

- [ ] Task 6.3 — Connection Cleanup
  - [ ] Detect dead WebSocket connections
  - [ ] Remove stale user sessions
  - [ ] Close goroutines safely
  - [ ] Test cleanup logic

---

## Phase 7 — Frontend (Minimal) ⏳ NOT STARTED

- [ ] Task 7.1 — Login Page
  - [ ] Simple login form
  - [ ] Send to POST /login
  - [ ] Store JWT token
  - [ ] Redirect on success

- [ ] Task 7.2 — Chat Window
  - [ ] Display chat messages
  - [ ] Send message form
  - [ ] Display online status
  - [ ] Show message history

- [ ] Task 7.3 — Group Chat UI
  - [ ] Minimal group list
  - [ ] Join group
  - [ ] Leave group
  - [ ] Group message display

---

## Phase 8 — Docker + Deployment ⏳ NOT STARTED

- [ ] Task 8.1 — Dockerize Backend
  - [ ] Create Dockerfile
  - [ ] Create docker-compose.yml
  - [ ] Services: app, postgres, redis
  - [ ] Test docker build and run

- [ ] Task 8.2 — Environment Variables
  - [ ] Use .env file
  - [ ] Load in docker-compose
  - [ ] Document all vars needed

- [ ] Task 8.3 — Deploy
  - [ ] Choose platform (Render/Railway/DigitalOcean)
  - [ ] Set up CI/CD (optional)
  - [ ] Deploy and test live

---

## Phase 9 — Resume Polish ⏳ NOT STARTED

- [ ] Task 9.1 — README
  - [ ] Architecture overview
  - [ ] Features list
  - [ ] Tech stack
  - [ ] Scaling discussion
  - [ ] API endpoints
  - [ ] Screenshots/demo

- [ ] Task 9.2 — Architecture Diagram
  - [ ] Frontend → Go Backend
  - [ ] Go Backend → Redis Pub/Sub
  - [ ] Redis ↔ PostgreSQL
  - [ ] WebSocket connections
  - [ ] Use Mermaid or Excalidraw

---

## Priority Checklist

### Must Finish (Blocking Interview)
- [x] JWT auth
- [ ] WebSocket read/write pumps
- [ ] Conversation-based message routing
- [ ] Message persistence
- [ ] Pagination
- [ ] Minimal deployment readiness

### Nice to Have (Polish)
- [ ] Read receipts
- [ ] Typing indicators
- [ ] File uploads
- [ ] Notifications
- [ ] Redis Pub/Sub
- [ ] Presence system

---

## 15-Day MVP Plan

- [x] Days 1-3 — Finish WebSocket core
  - [x] JWT-protected `/ws`
  - [x] Client and hub structs
  - [x] Read/write pumps

- [x] Days 4-7 — Message routing
  - [x] Conversation subscription model
  - [x] Send message through one conversation
  - [x] Broadcast only to participants

- [x] Days 8-10 — Persistence
  - [x] Save messages to PostgreSQL
  - [x] Load conversation history
  - [x] Basic pagination for history

- [ ] Days 11-13 — Basic UI / testing
  - [ ] Minimal chat window
  - [ ] Verify login → connect → send → receive
  - [ ] Test error cases

- [ ] Days 14-15 — Cleanup and demo prep
  - [ ] Fix edge cases
  - [ ] Update README
  - [ ] Prepare demo flow

---

## What NOT to Do

- ❌ Kubernetes
- ❌ Kafka
- ❌ Custom Redis implementation
- ❌ Microservices
- ❌ Complex frontend (React)
- ❌ Event sourcing
- ❌ CQRS

---

## Known Issues / Blockers

- [x] Main.go still calls `db.GetDB()` — need to verify GetDB function exists
- [x] Test signup/login endpoints with curl/Postman
- [x] Verify JWT_SECRET is set in .env (min 32 chars)
- [x] DNS resolution issue from WSL (if using WSL, may need DNS fix)

---

## Notes

- All Phase 0-1 tasks complete
- Phase 2 (auth) complete, ready to test
- Phase 3 (WebSockets) starts next — most critical for interview
- Redis is deferred until after the single-server MVP works
- Focus on clean code + architecture explanation over feature count
- Regularly commit to git with clear messages

---

## How to Use This File

1. Check current phase at top.
2. Mark task as complete when done: `[x]`
3. Update "Last Updated" date.
4. Use subtasks to break down work.
5. Commit with task reference: "Task 3.1: WebSocket upgrade handler"

---

Generated: 2026-05-19  
Roadmap Reference: PROJECT_ROADMAP.md
