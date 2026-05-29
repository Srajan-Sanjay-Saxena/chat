# Schema & Migrations (compact)

Schema (essential tables)
- `users(id uuid, username, password_hash, email, created_at)`
- `conversations(id uuid, title, created_at)`
- `conversation_participants(id uuid, conversation_id, user_id, created_at, unique(conversation_id,user_id))`
- `messages(id uuid, conversation_id, sender_id, content, created_at)`

Indexes
- `idx_messages_conv_created on messages(conversation_id, created_at desc)`
- participant indexes on conversation_id and user_id

Migrations
- Use versioned files: `0001_schema.up.sql` / `0001_schema.down.sql`.
- Prefer `IF NOT EXISTS` and `IF EXISTS` guards.
- For zero-downtime: add columns nullable then backfill, use `CONCURRENTLY` for indexes.

Test resets
- `helper.ResetSchema()` executes down then up under advisory lock to serialize test resets.
- Avoid concurrent DDL; use advisory locks or ephemeral schemas.

Sources
- Postgres advisory locks: https://www.postgresql.org/docs/current/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS