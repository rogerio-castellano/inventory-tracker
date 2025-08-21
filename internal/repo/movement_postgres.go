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
	return fmt.Errorf("failed to insert movement: %w", err)
}

const defaultLimit = 100

// GetByProductID returns all movements for a specific product
func (r *PostgresMovementRepository) GetByProductID(productID int, mf MovementFilter) ([]models.Movement, int, error) {
	// Build WHERE clause and collect arguments
	whereClause, args := r.buildWhereClause(productID, mf)

	// Handle special case: limit = 0 means return count only
	if mf.Limit != nil && *mf.Limit == 0 {
		total, err := r.getTotal(whereClause, args)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get total count: %w", err)
		}
		return []models.Movement{}, total, nil
	}

	// Validate offset
	if mf.Offset != nil && *mf.Offset < 0 {
		return nil, 0, fmt.Errorf("offset must be non-negative")
	}

	// Get total count
	total, err := r.getTotal(whereClause, args)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	// Early return if offset is beyond total
	if mf.Offset != nil && *mf.Offset >= total {
		return []models.Movement{}, total, nil
	}

	// Build and execute main query
	query, queryArgs := r.buildMainQuery(whereClause, args, mf)
	movements, err := r.executeQuery(query, queryArgs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute query: %w", err)
	}

	return movements, total, nil
}

// buildWhereClause constructs the WHERE clause and returns arguments
func (r *PostgresMovementRepository) buildWhereClause(productID int, mf MovementFilter) (string, []any) {
	args := []any{productID}
	whereClause := "WHERE product_id = $1"
	argIndex := 2

	if mf.Since != nil {
		whereClause += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, *mf.Since)
		argIndex++
	}

	if mf.Until != nil {
		whereClause += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, *mf.Until)
	}

	return whereClause, args
}

// buildMainQuery constructs the main SELECT query with pagination
func (r *PostgresMovementRepository) buildMainQuery(whereClause string, baseArgs []any, mf MovementFilter) (string, []any) {
	query := fmt.Sprintf("SELECT id, product_id, delta, created_at FROM movements %s ORDER BY created_at DESC", whereClause)
	args := make([]any, len(baseArgs))
	copy(args, baseArgs)
	argIndex := len(baseArgs) + 1

	// Apply limit
	limit := defaultLimit
	if mf.Limit != nil && *mf.Limit > 0 {
		limit = min(*mf.Limit, defaultLimit)
	}
	query += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, limit)
	argIndex++

	// Apply offset
	if mf.Offset != nil && *mf.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, *mf.Offset)
	}

	return query, args
}

// getTotal executes the count query
func (r *PostgresMovementRepository) getTotal(whereClause string, args []any) (int, error) {
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM movements %s", whereClause)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
}

// executeQuery executes the main query and scans results
func (r *PostgresMovementRepository) executeQuery(query string, args []any) ([]models.Movement, error) {
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

	// Check for iteration errors
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return movements, nil
}
