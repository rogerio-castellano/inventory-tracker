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

	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(AVG(price), 0) FROM products`).Scan(&m.AveragePrice)
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(price * quantity), 0) FROM products`).Scan(&m.TotalStockValue)
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity), 0) FROM products`).Scan(&m.TotalQuantity)

	// Top 5 movers
	rows, _ := r.db.QueryContext(ctx, `
			SELECT p.name, COUNT(*) AS cnt
			FROM movements m
			JOIN products p ON p.id = m.product_id
			GROUP BY p.name
			ORDER BY cnt DESC
			LIMIT 5
		`)
	defer rows.Close()
	for rows.Next() {
		var mover TopMover
		_ = rows.Scan(&mover.Name, &mover.Count)
		m.Top5Movers = append(m.Top5Movers, mover)
	}

	return m, nil
}
