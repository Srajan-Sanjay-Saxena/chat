package repository

import (
	"chat-v2/db"
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	// "errors"
)

var ErrUserExists = errors.New("user already exists")

func (r *Repository) CreateUser(ctx context.Context, user *db.User) error {
	// Writing SQL query to insert a new user into the database
	// query := `insert into users (id, username, password_hash, email, created_at) values ($1, $2, $3, $4, $5) returning id`
	query := fmt.Sprintf(`insert into %s (id, username, password_hash, email, created_at) values ($1, $2, $3, $4, $5) returning id`, r.table("users"))


	// Executing the query and scanning the returned id into the user struct
	err := r.DB.QueryRow(ctx, query, user.ID, user.Username, user.PasswordHash, user.Email, user.CreatedAt).Scan(&user.ID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrUserExists
		}
	}
	return err
}

func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*db.User, error) {
	// Writing SQL query to select a user by id
	// query := `select * from users where id = $1`
	query := fmt.Sprintf(`select * from %s where id = $1`, r.table("users"))

	// Executing the query and scanning the result into a user struct
	var user db.User
	err := r.DB.QueryRow(ctx, query, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*db.User, error) {

	// Writing SQL query to select a user by username
	// query := `select * from users where username = $1`
	query := fmt.Sprintf(`select * from %s where username = $1`, r.table("users"))

	// Executing the query and scanning the result into a user struct
	var user db.User
	err := r.DB.QueryRow(ctx, query, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) SearchUsers(ctx context.Context, q string, limit int) ([]*db.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	pattern := q + "%"
	// query := `select id, username from users where username ILIKE $1 order by username limit $2`
	query := fmt.Sprintf(`select id, username from %s where username ILIKE $1 order by username limit $2`, r.table("users"))

	rows, err := r.DB.Query(ctx, query, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*db.User
	for rows.Next() {
		var u db.User
		if err := rows.Scan(&u.ID, &u.Username); err != nil {
			return nil, err
		}
		users = append(users, &u)
	}
	return users, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*db.User, error) {
	// Writing SQL query to select a user by email
	// query := `select * from users where email = $1`
	query := fmt.Sprintf(`select * from %s where email = $1`, r.table("users"))

	// Executing the query and scanning the result into a user struct
	var user db.User
	err := r.DB.QueryRow(ctx, query, email).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
