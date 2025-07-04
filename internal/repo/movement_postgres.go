package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

type PostgresMovementRepository struct {
	db *sql.DB
}

func NewPostgresMovementRepository(db *sql.DB) *PostgresMovementRepository {
	return &PostgresMovementRepository{db: db}
}

// Log inserts a new inventory movement
func (r *PostgresMovementRepository) Log(productID, delta int) error {
	query := `INSERT INTO movements (product_id, delta, created_at, updated_at) VALUES ($1, $2, $3, $4)`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(ctx, query, productID, delta, time.Now().UTC(), time.Now().UTC())
	return err
}

// GetByProductID returns all movements for a specific product
func (r *PostgresMovementRepository) GetByProductID(productID int, since, until *time.Time) ([]models.Movement, error) {

	query := `SELECT id, product_id, delta, created_at FROM movements WHERE product_id = $1`
	args := []any{productID}
	idx := 2

	if since != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", idx)
		args = append(args, *since)
		idx++
	}
	if until != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", idx)
		args = append(args, *until)
		idx++
	}

	query += " ORDER BY created_at DESC"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movements []models.Movement
	for rows.Next() {
		var m models.Movement
		if err := rows.Scan(&m.ID, &m.ProductID, &m.Delta, &m.CreatedAt); err != nil {
			return nil, err
		}
		movements = append(movements, m)
	}
	return movements, nil
}
