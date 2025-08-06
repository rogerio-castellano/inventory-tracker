package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) GetByUsername(username string) (models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var u models.User
	err := r.db.QueryRowContext(ctx, `SELECT id, username, password_hash, role FROM users WHERE username = $1`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role)

	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrUserNotFound
	}
	return u, err
}

var ErrUserNotFound = errors.New("user not found")

func (r *PostgresUserRepository) CreateUser(u models.User) (models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id`
	err := r.db.QueryRowContext(ctx, query, u.Username, u.PasswordHash, u.Role).Scan(&u.ID)
	if err != nil {
		return models.User{}, err
	}
	return u, nil
}
