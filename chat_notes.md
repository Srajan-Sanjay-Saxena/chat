# Process Management Methods — Most Important & Frequently Used

## **PowerShell / Windows (Process Management)**

### **Core Methods**
| Method | Purpose |
|--------|---------|
| `Get-Process` | List all running processes |
| `Stop-Process` | Terminate a process by ID or name |
| `Start-Process` | Launch a new process |
| `Get-Process \| Where-Object` | Filter processes by criteria |
| `tasklist` | List processes (cmd alternative) |
| `taskkill` | Kill process (cmd alternative) |

### **Most Used Examples**
```powershell
# Get all processes
Get-Process

# Get specific process
Get-Process -Name "httpd" | Format-Table Name, Id, CPU, Memory   

# Get specific objects of all processes
Get-Process | Select-Object Name, Id, CPU, Memory

# Stop process by Process ID
Stop-Process -Id 5080 -Force

# Stop process by name
Stop-Process -Name "httpd" -Force

# List all processes on specific port
Get-NetTCPConnection -LocalPort 8080

# Force kill and wait
Stop-Process -Id 5080 -Force -ErrorAction SilentlyContinue

# start Process
Start-Process -FilePath "C:\path\to\app.exe"

Start-Process -FilePath "C:\path\to\app.exe" -ArgumentList "-arg1 value1"

Start-Process -FilePath "C:\path\to\app.exe" -WindowStyle Hidden

# WAIT FOR PROCESS
Wait-Process -Id 5080

Wait-Process -Name "httpd"

# LIST PROCESSES
tasklist
tasklist | find "httpd"
tasklist /PID 5080

# STOP PROCESS
taskkill /PID 5080
taskkill /PID 5080 /F
taskkill /IM httpd.exe /F
taskkill /IM httpd.exe /F /T

# START PROCESS
start C:\path\to\app.exe
start "" C:\path\to\app.exe arg1 arg2
```

---

## **Go Process Management**

The most popular choice is `github.com/joho/godotenv`.

What it does:
- Reads a .env file from your project root
- Parses lines like `PORT=8080` and `DB_HOST=localhost`
- Loads them into environment variables so you can read them with `os.Getenv()`

1. **Using Package after installing**
   ```go
    go get github.com/joho/godotenv
    import "github.com/joho/godotenv"
    func loadEnv() error {
        return godotenv.Load()
    }

    port := os.Getenv("PORT")
   ```

```go
// CREATE CONTEXT WITH TIMEOUT
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// CREATE CONTEXT WITH DEADLINE
deadline := time.Now().Add(30 * time.Second)
ctx, cancel := context.WithDeadline(context.Background(), deadline)
defer cancel()

// CREATE CANCELABLE CONTEXT
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// CHECK IF CONTEXT DONE
select {
case <-ctx.Done():
    fmt.Println("Context cancelled")
}

// GET DEADLINE
deadline, ok := ctx.Deadline()


// LAUNCH GOROUTINE
go functionName()

// WAIT FOR COMPLETION (WaitGroup)
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    // work
}()
wg.Wait()

// CHANNEL COMMUNICATION
done := make(chan bool)
go func() {
    done <- true
}()
<-done

// TIMEOUT ON CHANNEL
select {
case result := <-channel:
    fmt.Println(result)
case <-time.After(5 * time.Second):
    fmt.Println("Timeout")
}

// CREATE MULTIPLEXER
mux := http.NewServeMux()

// REGISTER HANDLER (function)
mux.HandleFunc("/path", handlerFunc)

// REGISTER HANDLER (interface)
mux.Handle("/path", handlerObject)

// SERVE HTTP
http.ListenAndServe(":8080", mux)

// SERVE HTTPS
http.ListenAndServeTLS(":443", "cert.pem", "key.pem", mux)
```
---

## Project Notes: Tasks 1.2, 1.3, 1.4 (cleaned, step-by-step)

Overview
- This document collects the decisions, SQL, and DB-access patterns used in phase‑1 of the Chat project.
- Sections are organized by task: schema design (1.2), migrations (1.3), and repository layer (1.4). Each section contains: what to do, why, exact syntax snippets, and short use-cases.

============================================================

Task 1.2 — Design Schema (step-by-step)

1) Goal
- Provide a normalized relational schema for chat functionality that is small and safe for phase‑1.

2) Required tables and minimal columns (SQL types shown)
- `users` (holds account info)
    - `id uuid PRIMARY KEY DEFAULT uuid_generate_v4()`
    - `username text NOT NULL UNIQUE`
    - `password_hash text NOT NULL`
    - `email text NOT NULL UNIQUE`
    - `created_at timestamptz NOT NULL DEFAULT now()`

- `conversations` (chat rooms or threads)
    - `id uuid PRIMARY KEY DEFAULT uuid_generate_v4()`
    - `title text` (optional)
    - `created_at timestamptz NOT NULL DEFAULT now()`

- `conversation_participants` (join table)
    - `id uuid PRIMARY KEY DEFAULT uuid_generate_v4()`
    - `conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE`
    - `user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE`
    - `created_at timestamptz NOT NULL DEFAULT now()`
    - `UNIQUE(conversation_id, user_id)`

- `messages` (chat content)
    - `id uuid PRIMARY KEY DEFAULT uuid_generate_v4()`
    - `conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE`
    - `sender_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE`
    - `content text NOT NULL`
    - `created_at timestamptz NOT NULL DEFAULT now()`

3) Rationale / why these choices
- Use join table for participants to avoid duplicating user data and to support many-to-many.
- Use `uuid` for stable cross-service IDs; `timestamptz` + `time.Time` in Go for timezone-safe timestamps.
- Keep fields minimal in phase‑1; add soft-delete, edited flags, attachments later via separate migrations.

Optional later tables (phase‑2 ideas)
- `message_reads` (receipt per user per message)
- `attachments` (file metadata, url, content type)

============================================================

Task 1.3 — Write SQL Migrations (step-by-step)

1) Goals / Subtasks
- Create versioned migration files that: create schema, add constraints, set timestamps, and add indexes needed for phase‑1 queries.

2) Minimal migration file layout (example filename)
- `db/0001_init.up.sql` — create tables + indexes
- `db/0001_init.down.sql` — drop tables (reverse order)

3) Important SQL syntax snippets (exact)

- Create extension for UUIDs (Postgres):
```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
```

- Create `users` table:
```sql
CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    username text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    email text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);
```

- Create `conversations` table:
```sql
CREATE TABLE IF NOT EXISTS conversations (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    title text,
    created_at timestamptz NOT NULL DEFAULT now()
);
```

- Create `conversation_participants` table:
```sql
CREATE TABLE IF NOT EXISTS conversation_participants (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(conversation_id, user_id)
);
```

- Create `messages` table (conversation-based):
```sql
CREATE TABLE IF NOT EXISTS messages (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
```

4) Indexes (important for queries/pagination)
```sql
CREATE INDEX IF NOT EXISTS idx_convpart_conv ON conversation_participants(conversation_id);
CREATE INDEX IF NOT EXISTS idx_convpart_user ON conversation_participants(user_id);
CREATE INDEX IF NOT EXISTS idx_messages_conv_created ON messages(conversation_id, created_at DESC);
```

5) Best-practice migration tips
- Use explicit columns in `INSERT` and `SELECT` to keep application mapping stable.
- Add indexes for frequent WHERE/JOIN columns (conversation_id, user_id, created_at).
- Keep migrations small and versioned — don't edit old migration files after they've been applied in shared environments.
- Down migrations should reverse changes in opposite order (drop messages -> participants -> conversations -> users).

============================================================

Task 1.4 — Repository Layer (step-by-step)

1) Goal
- Separate DB logic into repositories so handlers only orchestrate HTTP concerns. Repos own SQL and mapping to Go structs.

2) Recommended file structure
- `repository/repo.go` — interfaces only
- `repository/user_repo.go` — `pgUserRepo` implementation
- `repository/message_repo.go` — `pgMessageRepo` implementation
- `repository/conversation_repo.go` — `pgConversationRepo` implementation

3) Interface shape (exact signatures recommended)
```go
type UserRepository interface {
    CreateUser(ctx context.Context, u *db.User) error
    GetUserByID(ctx context.Context, id uuid.UUID) (*db.User, error)
    GetUserByUsername(ctx context.Context, username string) (*db.User, error)
}

type MessageRepository interface {
    CreateMessage(ctx context.Context, m *db.Message) error
    GetMessagesByConversationID(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*db.Message, error)
}

type ConversationRepository interface {
    CreateConversation(ctx context.Context, title string, creator uuid.UUID, participantIDs []uuid.UUID) (*db.Conversation, error)
    ListByUser(ctx context.Context, userID uuid.UUID) ([]*db.Conversation, error)
}
```

4) Why `context.Context` and `uuid.UUID`?
- `context.Context` (first param) ensures DB calls respect request cancellation and timeouts. Handlers pass `r.Context()`.
- `uuid.UUID` matches DB column types and prevents repeated parsing from strings.

5) Transaction example (verbal)
- When creating a conversation and inserting participant rows, begin a transaction with `pool.Begin(ctx)` or `pool.BeginTx(ctx, opts)`; run `tx.Exec` for each insert; `tx.Commit()` on success or `tx.Rollback()` on error.

6) Where to put SQL
- Keep SQL in repository implementations. Use `INSERT ... RETURNING id, created_at` to populate struct fields in one round-trip.

============================================================

pgx / pgxpool method reference (syntax + use case)

Note: `pool` below is `*pgxpool.Pool`, `ctx` is `context.Context`.

1) `pool.Query(ctx, sql, args...)` — multiple rows
- Syntax:
```go
rows, err := pool.Query(ctx, "SELECT id, username FROM users WHERE active=$1", true)
defer rows.Close()
for rows.Next() {
    var id uuid.UUID
    var username string
    if err := rows.Scan(&id, &username); err != nil { /* handle */ }
}
if err := rows.Err(); err != nil { /* handle */ }
```
- Use-case: listing users or fetching paginated messages where multiple rows are returned.

2) `pool.QueryRow(ctx, sql, args...)` — single row (or `INSERT ... RETURNING`)
- Syntax:
```go
row := pool.QueryRow(ctx, "INSERT INTO users (username, password_hash, email) VALUES ($1,$2,$3) RETURNING id, created_at", u.Username, u.PasswordHash, u.Email)
err := row.Scan(&u.ID, &u.CreatedAt)
if err == pgx.ErrNoRows { /* not found */ }
```
- Use-case: insert and get generated id, or fetch a single user by id.

3) `pool.Exec(ctx, sql, args...)` — execute without rows
- Syntax:
```go
tag, err := pool.Exec(ctx, "UPDATE users SET email=$1 WHERE id=$2", email, id)
affected := tag.RowsAffected()
```
- Use-case: updates and deletes where you only need affected-row count.

4) Transactions: `tx, err := pool.Begin(ctx)` and `tx.Commit()` / `tx.Rollback()`
- Syntax (verbal example): begin transaction, `tx.Exec` or `tx.QueryRow`, then `tx.Commit()` or `tx.Rollback()` on error.
- Use-case: create conversation + participants atomically; multi-statement changes that must all succeed.

5) `Scan` behavior (important details)
- `row.Scan(...)` and `rows.Scan(...)` map columns to destinations by position. The first selected column maps to the first destination argument, not by name.
- Always use explicit column lists in SQL so the order is deterministic. Avoid `SELECT *` in production code.
- Destinations must be pointers to compatible Go types (e.g., `*uuid.UUID`, `*time.Time`, `*string`).

6) Common error handling
- `err := row.Scan(...); if err == pgx.ErrNoRows { /* 404 handler */ } else if err != nil { /* internal error */ }`

7) Helpers / libraries
- For mapping rows to structs automatically consider `github.com/georgysavva/scany/pgxscan` or `github.com/jmoiron/sqlx` (with a pq/pgx driver) — they remove manual `Scan` boilerplate.

============================================================


Why errors.Is is better
It works with wrapped errors.
It is the standard Go way to check for a sentinel error like ErrUserExists.
It is more stable than comparing error text.
Why direct comparison is weaker
Bad pattern:
```go
if err.Error() == "sql: no rows in result set" {}
```
Problems:

error messages can change
wrapped errors will not match
you are comparing strings, not error identity
Better pattern
Define a sentinel error:
Return it from repo:

Check it in handler:


### ERRORS
Preserve original errors with wrapping
Right now you build new strings like failed to parse token: + err.Error().
This loses error identity, so you cannot use errors.Is for checks like token expired.
Use wrapping style with %w (fmt.Errorf) so callers can detect JWT errors.

---


---

## Phase 3 — WebSocket Core (checklist)

This checklist collects the concrete tasks, guardrails and best-practices for the WebSocket layer (Phase 3).

- **Upgrade & Auth**: verify JWT at the HTTP -> WS upgrade. Reject malformed or expired tokens immediately.
- **Origin allowlist**: enforce `isAllowedOrigin` for browser clients; keep a configurable allowlist for deployments.
- **DI for membership checks**: inject a small `isParticipant(ctx, conversationID, userID) (bool,error)` function from `main` into the WS handler and store it on `client`.
- **Hub-owned subscription state**: keep `subscribedConversations` owned and mutated only inside the hub `Run()` loop to avoid data races.
- **Structured acks**: send non-blocking JSON acks for `subscribe`/`unsubscribe`/`publish` attempts: `subscribed`, `unsubscribed`, `error` (with reason).
- **Authorization on actions**: hub must verify membership on subscribe and reject publish attempts if sender is not subscribed/participant.
- **Request-scoped timeouts**: call `isParticipant` and other DB operations with `context.WithTimeout` (e.g., 200ms–2s depending on latency SLA) to keep hub responsive.
- **Non-blocking sends / backpressure**: always send via `select { case ch<-msg: default: }` for acks; disconnect or drop slow clients and record metrics.
- **Persistence ordering**: prefer save-then-broadcast so message IDs/timestamps are authoritative; if using async persistence, ensure idempotency.
- **Message validation & limits**: validate types, enforce `maxMsgSize`, field lengths, and whitelist allowed JSON fields before processing.
- **Rate limiting & abuse prevention**: throttle publish/subscribe per socket and per conversation; consider burst tokens and punish repeated violations.
- **Graceful shutdown**: implement `hub.Stop()` and `hub.Done()` to close clients and drain queues on shutdown (already added). Ensure `client.close()` is idempotent.
- **Reconnect behavior**: on reconnect, either auto-subscribe by querying `GetConversationsByUserID` or make the client re-subscribe and provide endpoints to fetch missed messages.
- **Testing**: add unit tests for hub state transitions and integration tests for end-to-end publish/subscribe + persistence.
- **Metrics & logging**: capture publish latency, subscribe failures, slow-client disconnects, DB timeout counts, and ack latency for observability.
- **Security**: never trust client-supplied sender IDs; use the authenticated `userID`. Sanitize logs to avoid recording raw message content.
- **Scaling plan**: single-node hub for MVP; plan Redis/External Pub/Sub for cross-instance fan-out later.

Add or refine items here as Phase 3 work completes.

errors.New() makes new error we cannot use errors.Is but fmt.Errorf("failed to parse token: %w", err) 
preserves identity and also can add info


Handler: NewWebSocketHandler in ws_handler.go: JWT check (Authorization Bearer), WebSocket upgrade, creates client, registers with hub.
Hub core: Hub in structure.go: single hub, register/unregister, subscribe/unsubscribe, broadcast channels and Run() loop that routes by conversationID and performs participant checks.
Client shape: client struct in structure.go: connection, buffered send channel, subscribedConversations, userID, hub reference, and isParticipant hook.
Pumps: readPump() and writePump() in pumping.go: JSON parsing, message/subscribe/unsubscribe handling, ping/pong, read/write deadlines, send buffering, unregister on read failure, logging.
Wiring: main.go creates hub, starts it (go hub.Run()), and registers ws handler with an IsParticipant callback from the repository.


Pump lifetime ownership: ensure both pumps unregister the client and close resources exactly once (make unregister idempotent). Currently readPump unregisters; make writePump also trigger unregister on write failures.
Read setup placement: move SetReadLimit, SetReadDeadline, and SetPongHandler to before the read loop (now set after reading).
Graceful hub shutdown: add stop/context to Hub so main can close all clients on server shutdown.
Persistence: wire message persistence (repository CreateMessage). decide ordering: persist-before-broadcast (safer) or broadcast-then-persist (lower latency) and implement retries/queue if async.
Subscription concurrency guarantee: current design mutates subscribedConversations inside hub (good). Ensure no client goroutine writes it directly elsewhere.
Unregister/close semantics: ensure client.send is closed exactly once and pumps exit reliably without goroutine leaks.
Backpressure policy & metrics: instrument drops (when send full), consider strategies (drop oldest, disconnect, or backpressure).
Security & origin: tighten Upgrader.CheckOrigin for prod and consider token refresh/expiry handling on reconnect.

---

## Phase 3 WebSocket Core - Detailed Notes

### Status
- Core WebSocket phase is complete for the current single-server MVP scope.
- The essential runtime path now exists end to end: authenticated upgrade, client registration, subscribe/unsubscribe routing, message persistence, broadcast, cleanup, and graceful shutdown.
- Remaining items are hardening tasks rather than blockers for the current phase: richer metrics, more origin policy automation, and broader load/resilience testing.

### What Changed In The `ws` Package
- `ws/ws_handler.go`
    - Accepts the authenticated HTTP request and upgrades it to a WebSocket connection.
    - Validates the `Authorization: Bearer ...` header before upgrading.
    - Checks the JWT and extracts the user ID for the socket session.
    - Uses `CheckOrigin` with a deployment-aware allowlist driven from config.
    - Creates the client, registers it with the hub, and starts the pumps.
- `ws/structure.go`
    - Defines the client state: connection, buffered send channel, user ID, subscribed conversations, hub reference, and participant checker.
    - Defines the hub state: connected clients, register/unregister channels, subscribe/unsubscribe channels, broadcast channel, and shutdown channels.
    - Owns all subscription mutation inside the hub loop so client goroutines do not race on shared state.
    - Makes `client.close()` idempotent so cleanup can be triggered from multiple paths safely.
    - Adds `Hub.Stop()` and `Hub.Done()` so shutdown can be coordinated from `main.go`.
    - Disconnects slow clients when their send buffer is full instead of letting memory grow without bound.
- `ws/pumping.go`
    - `readPump()` now sets read limits, deadlines, and pong handler before entering the read loop.
    - The read loop handles three message types: `message`, `subscribe`, and `unsubscribe`.
    - Message creation now goes through a message service so the socket path does not write directly to the repository.
    - The saved message returned from the database is what gets broadcast, so the payload matches persisted data exactly.
    - `writePump()` sends pings, writes messages, and triggers the unified close path on any failure.
- `ws/ws_integration_test.go`
    - Adds a real integration test that exercises auth, allowed origin checks, subscription, publish, broadcast, and database persistence.
    - Uses `httptest.Server` and a real `websocket.Dialer` rather than unit-only mocks.
    - Verifies the saved message is the same record that is delivered back over the socket.

### Runtime Flow
1. The client sends an authenticated HTTP request to `/ws`.
2. The handler validates the bearer token and upgrades the request.
3. The new client is registered in the hub and its pumps start.
4. Subscribe and unsubscribe requests are processed by the hub, not by the client goroutine.
5. A publish request is validated, checked for membership, persisted, and then broadcast using the saved row.
6. The write pump delivers outbound messages and disconnects on write or ping failure.
7. The close path is idempotent so either pump can initiate shutdown without double-closing resources.
8. `main.go` stops the hub on server shutdown and waits for all hub cleanup to finish before closing the database.

### Important Design Decisions
- Persist before broadcast.
    - This keeps message IDs, timestamps, and broadcast payloads aligned with the database.
    - It is the safer choice for the current MVP because the server remains the source of truth.
- Keep subscription ownership inside the hub.
    - This avoids client-side data races on `subscribedConversations`.
    - The hub is the only place that mutates subscription state.
- Keep the system single-server for now.
    - Redis is intentionally deferred until the single-node flow is stable.
    - That keeps the current phase smaller and easier to reason about.
- Use deployment-specific origin allowlists.
    - Local development can still work without extra setup.
    - Production deployments can set `WS_ALLOWED_ORIGINS` to exact frontend domains.

### Validation That Passed
- Static checks on all touched files returned no errors.
- The focused websocket integration test passed.
- The test verified subscribe, publish, broadcast, and persistence against the real stack.

### Remaining Hardening Items
- Add metrics for slow-client disconnects and dropped sends if traffic grows.
- Expand origin configuration docs for local, Railway, Vercel, and custom-domain deployments.
- Add a second integration test for unauthorized origin or invalid token rejection.
- Add a small reconnect/expiry test if you want to document JWT expiry behavior more explicitly.
- Add broader load testing only after the current socket flow is stable under normal usage.

### Practical Summary
- The WebSocket phase is effectively done for the current goal of a simple single-server chat MVP.
- The code now has the pieces needed to support the next phase of product work without revisiting the socket foundation.
- The remaining work is mostly operational polish and scale hardening, not core correctness.


## Message Routing — Detailed Plan

This section contains a step-by-step plan for Phase 4: Message routing (client → hub → subscribers → persistence). Each step below maps to the project TODOs and includes implementation notes and acceptance criteria.

1) Design message routing model (goal & acceptance)
    - Goal: Define how messages flow from a connected client, through the Hub, to conversation subscribers and the persistence layer.
    - Acceptance: Sequence diagram + small prototype showing a single message routed to N participants.

2) Define wire + DB message schema
    - Wire (WebSocket) envelope: { type, conversation_id, message_id, sender_id, payload, created_at, meta }
    - DB row: include conversation_id, sender_id, content, created_at, delivered/acked flags (optional separate table)
    - Acceptance: JSON schema and SQL migration agreed and committed.

3) Conversation subscription management
    - Maintain map[conversation_id] -> set(clientIDs) and client -> set(conversationIDs).
    - Ensure concurrency-safe access (sync.RWMutex or sharded maps) and consider per-conversation goroutine for routing.
    - Acceptance: APIs to Subscribe/Unsubscribe tested with concurrent join/leave.

4) Implement Hub routing (register/unregister)
    - Hub responsibilities: register/unregister clients, accept inbound messages, route to subscribers, broadcast control messages.
    - Use channels for register/unregister/inbound; keep select loop focused and small.
    - Acceptance: Hub unit tests for register/unregister and broadcast to multiple clients.

5) Client subscription mapping to conversations
    - Client struct holds `send chan []byte`, `userID`, `subscriptions map[uuid]bool`.
    - On upgrade, validate JWT, create Client, register with Hub, optionally auto-subscribe to recent conversations.
    - Acceptance: Clean client lifecycle with no goroutine leaks after disconnect.

6) Delivery guarantees and acknowledgements
    - Decide default: at-most-once for MVP; design hooks for at-least-once (message IDs + ACKs) later.
    - Define ACK envelope and lightweight retry/backoff policy; consider idempotency keys for DB writes.
    - Acceptance: ACK round-trip for a message in integration test (optional for MVP but planned).

7) Persistence pipeline and DB integration
    - Accept messages in Hub, enqueue to a worker pool for async DB insert (to avoid blocking broadcast).
    - Use buffered channel / bounded worker pool; on persistent failures push to retry queue or DLQ.
    - Acceptance: Messages are saved within N seconds in integration test; errors retried X times.

8) Error handling, retries, dead-letter queue
    - Centralize error types; on transient DB error retry, on permanent error log and move to DLQ table.
    - Emit metrics for failures and delivery latency.
    - Acceptance: Retries observed and DLQ entries created for persistent failures.

9) Heartbeats, presence & connection cleanup
    - Implement ping/pong in write-pump; detect stale connections and remove from Hub.
    - Cleanup: close send channel, unregister client, remove subscriptions, stop goroutines.
    - Acceptance: Simulated stale connection removed and resources freed.

10) Testing: unit, integration, load tests
    - Unit test Hub logic, client lifecycle, subscription maps.
    - Integration test: login -> connect /ws -> send message -> other client receives -> DB contains message.
    - Load test: simulate 1000 concurrent clients sending messages; measure latency and memory.
    - Acceptance: basic integration tests pass locally; load test results recorded.

11) Documentation, examples, and diagrams
    - Add sequence diagrams (Mermaid) and a short example client script (JS or Go) demonstrating connect/send/ack.
    - Acceptance: docs committed and referenced from README.

12) Study topics & recommended books
    - See STUDY_TOPICS.md for curated reading and exercises to deepen understanding.

---

Refer to STUDY_TOPICS.md for reading material and practical exercises.
