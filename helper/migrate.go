package helper

import (
	"chat-v2/db"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaResetLockKey int64 = 842019

func schemaSQLPath(filename string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("..", "db", filename)
	}
	helperDir := filepath.Dir(file)
	return filepath.Join(helperDir, "..", "db", filename)
}

func executeSQLFile(ctx context.Context, conn *pgxpool.Conn, filename string) error {
	schema, err := os.ReadFile(schemaSQLPath(filename))
	if err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, string(schema)); err != nil {
		return err
	}

	return nil
}

func withSchemaResetLock(ctx context.Context, fn func(context.Context, *pgxpool.Conn) error) error {
	conn, err := db.GetDB().Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `select pg_advisory_lock($1)`, schemaResetLockKey); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `select pg_advisory_unlock($1)`, schemaResetLockKey)
	}()

	return fn(ctx, conn)
}

func Migrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return withSchemaResetLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		return executeSQLFile(ctx, conn, "0001_schema.up.sql")
	})
}

func Rollback() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return withSchemaResetLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		return executeSQLFile(ctx, conn, "0001_schema.down.sql")
	})
}

func ResetSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return withSchemaResetLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		if err := executeSQLFile(ctx, conn, "0001_schema.down.sql"); err != nil {
			return fmt.Errorf("rollback schema: %w", err)
		}
		if err := executeSQLFile(ctx, conn, "0001_schema.up.sql"); err != nil {
			return fmt.Errorf("migrate schema: %w", err)
		}
		return nil
	})
}
