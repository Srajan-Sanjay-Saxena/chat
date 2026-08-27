package postgres

import (
	"chat-v2/internal/domain/ent"
	"context"
	"time"

	"entgo.io/ent/dialect"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewClient(dsn string) (*ent.Client, error) {
	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Schema.Create(ctx); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}
