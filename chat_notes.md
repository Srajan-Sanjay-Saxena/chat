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

errors.New() makes new error we cannot use errors.Is but fmt.Errorf("failed to parse token: %w", err) 
preserves identity and also can add info
