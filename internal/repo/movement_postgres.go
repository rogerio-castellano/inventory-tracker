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
func (r *PostgresMovementRepository) GetByProductID(productID int, since, until *time.Time, limit, offset *int) ([]models.Movement, int, error) {
	query := `SELECT id, product_id, delta, created_at FROM movements WHERE product_id = $1`
	countQuery := `SELECT COUNT(*) FROM movements WHERE product_id = $1`

	args := []any{productID}
	countArgs := []any{productID}
	idx := 2

	if since != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", idx)
		countQuery += fmt.Sprintf(" AND created_at >= $%d", idx)
		args = append(args, *since)
		countArgs = append(countArgs, *since)
		idx++
	}
	if until != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", idx)
		countQuery += fmt.Sprintf(" AND created_at <= $%d", idx)
		args = append(args, *until)
		countArgs = append(countArgs, *until)
		idx++
	}

	query += " ORDER BY created_at DESC"

	if limit != nil && *limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", idx)
		args = append(args, limit)
		idx++
	}
	if offset != nil {
		query += fmt.Sprintf(" OFFSET $%d", idx)
		args = append(args, offset)
		idx++
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Count total
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// If no movements, return early
	if total == 0 {
		return []models.Movement{}, 0, nil
	}

	// If limit is zero, return early with empty data
	// This is to handle cases where we want to count but not fetch any rows
	if limit != nil && *limit == 0 {
		return []models.Movement{}, total, nil
	}

	// If the offset is greater than the total, return only metadata with no movements
	// This is to handle pagination where the offset exceeds available data
	if offset != nil && *offset >= total {
		return []models.Movement{}, total, nil
	}

	defaultLimit := 3 // Default limit if none provided

	// If no limit is provided or if the limit exceeds the default, apply the default limit
	if limit == nil || *limit > defaultLimit {
		query += fmt.Sprintf(" LIMIT $%d", idx)
		args = append(args, defaultLimit)
	}

	// Fetch movements
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var movements []models.Movement
	for rows.Next() {
		var m models.Movement
		if err := rows.Scan(&m.ID, &m.ProductID, &m.Delta, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		movements = append(movements, m)
	}
	return movements, total, nil
}
