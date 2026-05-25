package repository

import (
	"chat-v2/db"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	// "errors"
)

var ErrUserExists = errors.New("user already exists")

func (r *Repository) CreateUser(ctx context.Context, user *db.User) error {
	// Writing SQL query to insert a new user into the database
	query := `insert into users (id, username, password_hash, email, created_at) values ($1, $2, $3, $4, $5) returning id`

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
	query := `select * from users where id = $1`

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
	query := `select * from users where username = $1`

	// Executing the query and scanning the result into a user struct
	var user db.User
	err := r.DB.QueryRow(ctx, query, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
