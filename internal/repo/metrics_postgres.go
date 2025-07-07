package repo

import (
	"context"
	"database/sql"
	"time"
)

type PostgresMetricsRepository struct {
	db *sql.DB
}

func NewPostgresMetricsRepository(db *sql.DB) *PostgresMetricsRepository {
	return &PostgresMetricsRepository{db: db}
}

func (r *PostgresMetricsRepository) GetDashboardMetrics() (Metrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var m Metrics

	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM products`).Scan(&m.TotalProducts)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM movements`).Scan(&m.TotalMovements)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM products WHERE quantity < threshold`).Scan(&m.LowStockCount)

	_ = r.db.QueryRowContext(ctx, `
		SELECT p.name, COUNT(*) as cnt
		FROM movements m
		JOIN products p ON m.product_id = p.id
		GROUP BY p.name
		ORDER BY cnt DESC
		LIMIT 1
	`).Scan(&m.MostMovedProduct.Name, &m.MostMovedProduct.MovementCount)

	return m, nil
}
