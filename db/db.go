package db

import (
	"context"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

type Db = *pgxpool.Pool

func Connect(dbSource string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(dbSource)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

// maskDSN returns a version of the DSN with the password redacted.
// It preserves the username, host, and path for logging purposes.
func maskDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "<invalid dsn>"
	}
	if u.User != nil {
		name := u.User.Username()
		u.User = url.UserPassword(name, "****")
	}
	// Avoid returning credentials; include host and path only
	out := u.Scheme + "://"
	if u.User != nil {
		out += u.User.Username() + "@"
	}
	out += u.Host + u.Path
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	return out
}

func GetDB() *pgxpool.Pool {
	return DB
}
