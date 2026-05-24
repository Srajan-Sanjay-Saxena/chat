# Test DB reset & migration (compact)

Internal-only notes: keep this file out of GitHub-facing documentation.

Context:
- Tests need a clean DB schema for reliable integration testing.
- Existing helper: `helper.ResetSchema()` executes `0001_schema.down.sql` then `0001_schema.up.sql` under an advisory lock.

Problem encountered (23-05-2026):
- Running rollback + migrate in tests produced catalog errors (duplicate type/name) — concurrent DDL or leftover objects caused SQLSTATE 23505.

Attempts & resolution:
- Tried running the migration SQL inside a single transaction to avoid mid-state visibility. That produced other timing/locking issues in the test harness.
- Reverted the transactional experiment and ensured the helper uses an advisory lock to serialize resets.
- Final local run: successful after ensuring tests acquire the same advisory lock before ResetSchema.

Best practices:
- Use an exclusive test DB or unique schema per test suite when possible.
- Serialize DDL operations with advisory locks (as this project does) or use ephemeral databases.
- Keep migrations idempotent where possible: `IF NOT EXISTS` / `IF EXISTS` guards in DDL.

Sources:
- pg advisory locks: https://www.postgresql.org/docs/current/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS
- Zero-downtime migration patterns (ideas for safer DDL): see project docs in `docs/migrations.md` (suggested).