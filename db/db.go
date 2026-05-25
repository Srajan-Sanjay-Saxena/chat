package db

import(
	"context"
	"net/url"
	"time"
	"github.com/jackc/pgx/v5/pgxpool"
	"chat-v2/logger"
)

var DB *pgxpool.Pool

// Connect establishes a connection to the PostgreSQL database using the provided DSN.
// It uses a context with a timeout to avoid hanging indefinitely if the database is unreachable.
func Connect(dbSource string) error {
	// Create a context with a timeout for the connection attempt
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Log a masked version of the DSN (avoid leaking credentials)
	masked := maskDSN(dbSource)
	logger.Log.Info("Attempting database connection", "db", masked)

	// Connect to the database
	pool, err := pgxpool.New(ctx, dbSource)
	if err != nil {
		return err
	}

	// Ping the database to verify the connection
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	// Assign the pool to the global variable
	DB = pool
	return nil
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