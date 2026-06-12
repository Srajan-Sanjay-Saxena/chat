package helper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"chat-v2/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)


func schemaSQLPath(filename string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("..", "db", filename)
	}
	helperDir := filepath.Dir(file)
	return filepath.Join(helperDir, "..", "db", filename)
}

func executeSQLScript(ctx context.Context, conn *pgxpool.Conn, script string) error {
	statements := strings.Split(script, ";")
	for _, statement := range statements {
		trimmed := strings.TrimSpace(statement)
		if trimmed == "" {
			continue
		}
		if _, err := conn.Exec(ctx, trimmed); err != nil {
			return err
		}
	}
	return nil
}

func executeSQLFile(ctx context.Context, conn *pgxpool.Conn, filename string) error {
	schema, err := os.ReadFile(schemaSQLPath(filename))
	if err != nil {
		return err
	}

	return executeSQLScript(ctx, conn, string(schema))
}

// executeSQLFileIfExists reads and executes the SQL file only if it exists.
// This allows optional migrations to be applied when present.
func executeSQLFileIfExists(ctx context.Context, conn *pgxpool.Conn, filename string) error {
	path := schemaSQLPath(filename)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return executeSQLFile(ctx, conn, filename)
}


func Migrate(pool *pgxpool.Pool, schema string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, fmt.Sprintf(`SET search_path TO "%s",public`, schema)); err != nil {
		return err
	}

	if err := executeSQLFileIfExists(ctx, conn, "0001_schema.up.sql"); err != nil {
		return err
	}

	return nil
}

func Rollback(pool *pgxpool.Pool, schema string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, fmt.Sprintf(`SET search_path TO "%s",public`, schema)); err != nil {
		return err
	}

	if err := executeSQLFileIfExists(ctx, conn, "0001_schema.down.sql"); err != nil {
		return err
	}

	return nil
}

func ResetSchema(DB *pgxpool.Pool, schemaName string) error {
	logger.Log.Info("Resetting database schema", "schema", schemaName)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	conn, err := DB.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	_, err = tx.Exec(ctx,
		fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName))
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		fmt.Sprintf(`CREATE SCHEMA "%s"`, schemaName))
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		fmt.Sprintf(`SET search_path TO "%s",public`, schemaName))
	if err != nil {
		return err
	}

	statements := []string{
		`drop table if exists conversation_participants cascade`,
		`drop table if exists messages cascade`,
		`drop table if exists conversations cascade`,
		`drop table if exists users cascade`,
		`create extension if not exists "uuid-ossp"`,
		`create table if not exists users (
		id uuid primary key default uuid_generate_v4(),
		username text unique not null,
		password_hash text not null,
		email text unique not null,
		created_at timestamptz not null default now()
	)`,
		`create table if not exists conversations (
		id uuid primary key default uuid_generate_v4(),
		type text not null default 'group',
		title text,
		display_name text,
		canonical_name text,
		created_at timestamptz not null default now()
	)`,
		`create unique index if not exists idx_conversations_canonical_private on conversations(canonical_name) where (type = 'private')`,
		`create table if not exists conversation_participants (
		id uuid primary key default uuid_generate_v4(),
		conversation_id uuid not null references conversations(id) on delete cascade,
		user_id uuid not null references users(id) on delete cascade,
		created_at timestamptz not null default now(),
		unique (conversation_id, user_id)
	)`,
		`create table if not exists messages (
		id uuid primary key default uuid_generate_v4(),
		conversation_id uuid not null references conversations(id) on delete cascade,
		sender_id uuid not null references users(id) on delete cascade,
		sender_username text not null,
		content text not null,
		created_at timestamptz not null default now()
	)`,
		`create index if not exists idx_convpart_conv on conversation_participants(conversation_id)`,
		`create index if not exists idx_convpart_user on conversation_participants(user_id)`,
		`create index if not exists idx_messages_conv_created on messages(conversation_id, created_at desc)`,
	}

	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("reset schema: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("reset schema commit: %w", err)
	}

	return nil
}
