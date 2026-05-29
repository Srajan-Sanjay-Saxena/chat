package helper

import (
	"chat-v2/db"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	return withSchemaResetLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		if err := executeSQLFileIfExists(ctx, conn, "0001_schema.up.sql"); err != nil {
			return err
		}
		if err := executeSQLFileIfExists(ctx, conn, "0002_add_conversation_type.up.sql"); err != nil {
			return err
		}
		if err := executeSQLFileIfExists(ctx, conn, "0003_add_canonical_name.up.sql"); err != nil {
			return err
		}
		return nil
	})
}

func Rollback() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	return withSchemaResetLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		if err := executeSQLFileIfExists(ctx, conn, "0003_add_canonical_name.down.sql"); err != nil {
			return err
		}
		if err := executeSQLFileIfExists(ctx, conn, "0002_add_conversation_type.down.sql"); err != nil {
			return err
		}
		if err := executeSQLFileIfExists(ctx, conn, "0001_schema.down.sql"); err != nil {
			return err
		}
		return nil
	})
}

func ResetSchema() error {
	return withSchemaResetLock(context.Background(), func(ctx context.Context, conn *pgxpool.Conn) error {
		// Use a generous timeout for schema operations
		ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()

		statements := []string{
			`drop table if exists public.conversation_participants cascade`,
			`drop table if exists public.messages cascade`,
			`drop table if exists public.conversations cascade`,
			`drop table if exists public.users cascade`,
			`create extension if not exists "uuid-ossp"`,
			`create table if not exists public.users (
			id uuid primary key default uuid_generate_v4(),
			username text unique not null,
			password_hash text not null,
			email text unique not null,
			created_at timestamptz not null default now()
		)`,
			`create table if not exists public.conversations (
			id uuid primary key default uuid_generate_v4(),
			type text not null default 'group',
			title text,
			display_name text,
			canonical_name text,
			created_at timestamptz not null default now()
		)`,
			`create unique index if not exists idx_conversations_canonical_private on public.conversations(canonical_name) where (type = 'private')`,
			`create table if not exists public.conversation_participants (
			id uuid primary key default uuid_generate_v4(),
			conversation_id uuid not null references public.conversations(id) on delete cascade,
			user_id uuid not null references public.users(id) on delete cascade,
			created_at timestamptz not null default now(),
			unique (conversation_id, user_id)
		)`,
			`create table if not exists public.messages (
			id uuid primary key default uuid_generate_v4(),
			conversation_id uuid not null references public.conversations(id) on delete cascade,
			sender_id uuid not null references public.users(id) on delete cascade,
			content text not null,
			created_at timestamptz not null default now()
		)`,
			`create index if not exists idx_convpart_conv on public.conversation_participants(conversation_id)`,
			`create index if not exists idx_convpart_user on public.conversation_participants(user_id)`,
			`create index if not exists idx_messages_conv_created on public.messages(conversation_id, created_at desc)`,
		}

		for _, statement := range statements {
			if _, err := conn.Exec(ctx, statement); err != nil {
				return fmt.Errorf("reset schema: %w", err)
			}
		}

		return nil
	})
}
