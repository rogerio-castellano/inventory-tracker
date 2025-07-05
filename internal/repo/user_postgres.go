package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

type UserRepository interface {
	GetByUsername(username string) (models.User, error)
}

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
	err := r.db.QueryRowContext(ctx, `SELECT id, username, password_hash FROM users WHERE username = $1`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash)

	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrUserNotFound
	}
	return u, err
}

var ErrUserNotFound = errors.New("user not found")
