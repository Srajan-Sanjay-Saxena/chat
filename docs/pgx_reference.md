# pgx / pgxpool Reference (compact)

Common calls
- `pool.Query(ctx, sql, args...)` — multiple rows
- `pool.QueryRow(ctx, sql, args...)` — single row / `RETURNING`
- `pool.Exec(ctx, sql, args...)` — no rows

Transactions
- `tx, err := pool.Begin(ctx)`; `tx.Exec` / `tx.QueryRow`; `tx.Commit()`

Scan notes
- `rows.Scan(&a,&b)` maps by position; use explicit column lists.

Paging example
- Cursor encode `created_at + id` for stable ordering; encode to base64 JSON for opaque cursor.

Tools
- Consider `scany/pgxscan` or `sqlx` for mapping rows to structs.

