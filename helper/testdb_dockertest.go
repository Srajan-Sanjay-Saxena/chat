package helper

import (
	"fmt"
	"strings"

	"chat-v2/db"

	"github.com/google/uuid"
	"github.com/ory/dockertest/v3"
)

// StartTestDB spins up a Postgres container (dockertest), connects the global DB,
// runs migrations and returns the pool and a cleanup function.
// Minimal, intended for tests: call cleanup() in a defer in your TestMain or test.
func StartTestDB() (*pgxpoolType, func() error, error) {
	// Use an opaque type for pgxpool to avoid importing pgx here repeatedly
	// but we need to return the pool. Instead, callers can use db.GetDB().
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, fmt.Errorf("could not create docker pool: %w", err)
	}

	dbName := "chat_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15-alpine",
		Env: []string{
			"POSTGRES_USER=postgres",
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_DB=" + dbName,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not start postgres container: %w", err)
	}

	// exponential backoff to wait for container
	if err := pool.Retry(func() error {
		port := resource.GetPort("5432/tcp")
		dsn := fmt.Sprintf("postgres://postgres:secret@localhost:%s/%s?sslmode=disable", port, dbName)
		return db.Connect(dsn)
	}); err != nil {
		_ = pool.Purge(resource)
		return nil, nil, fmt.Errorf("could not connect to postgres: %w", err)
	}

	// Apply migrations using helper.Migrate
	if err := Migrate(); err != nil {
		// clean up
		if db.GetDB() != nil {
			db.GetDB().Close()
		}
		pool.Purge(resource)
		return nil, nil, fmt.Errorf("migrate failed: %w", err)
	}

	cleanup := func() error {
		// close global pool
		if db.GetDB() != nil {
			db.GetDB().Close()
		}
		return pool.Purge(resource)
	}

	// Return nil for explicit pool type; callers should use db.GetDB()
	return nil, cleanup, nil
}

// Minimal forward-declaration to avoid importing pgxpool in this file; callers
// should use db.GetDB() for the actual pool. This type alias is here to keep
// the function signature explicit while keeping this helper minimal.
type pgxpoolType struct{}
