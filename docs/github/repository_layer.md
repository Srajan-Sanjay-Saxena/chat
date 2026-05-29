# Repository Layer (compact)

Purpose
- Encapsulate SQL and mapping; handlers only orchestrate.
- Use `context.Context` and `uuid.UUID` in signatures.

Recommended interfaces
- `CreateUser(ctx, *db.User) error`
- `CreateMessage(ctx, *db.Message) error`
- `GetMessagesByConversationID(ctx, uuid, *string, limit) (*MessageResponse, error)`

Transactions
- Use `tx, err := pool.Begin(ctx)` for multi-statement operations (create conversation + participants).
- Always `defer tx.Rollback(ctx)` and `tx.Commit(ctx)` on success.

Best practices
- Use `INSERT ... RETURNING` to get generated ids/timestamps.
- Avoid `SELECT *`; list columns explicitly to keep `Scan` stable.
- Keep SQL in repository implementations.