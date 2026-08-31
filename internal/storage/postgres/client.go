package postgres

import (
	"context"
	"database/sql"
	"time"

	"entgo.io/ent/dialect"
	_ "github.com/jackc/pgx/v5/stdlib"
	entsql "entgo.io/ent/dialect/sql"

	"chat-v2/internal/domain/ent"
)

func NewClient(dsn string) (*ent.Client, error) {
	// Open
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	// wrap the sql.DB with ent's client
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := ent.NewClient(ent.Driver(drv))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Schema.Create(ctx); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}
