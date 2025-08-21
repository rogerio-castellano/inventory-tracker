package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

type PostgresProductRepository struct {
	db *sql.DB
}

func NewPostgresProductRepository(db *sql.DB) *PostgresProductRepository {
	return &PostgresProductRepository{db: db}
}

func (r *PostgresProductRepository) Create(p models.Product) (models.Product, error) {
	query := `INSERT INTO products (name, price, quantity, threshold, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.db.QueryRowContext(ctx, query, p.Name, p.Price, p.Quantity, p.Threshold, p.CreatedAt, p.UpdatedAt).Scan(&p.ID)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			err = fmt.Errorf("%w: %v", ErrDuplicatedValueUnique, err)
		}
	}

	return p, err
}

func (r *PostgresProductRepository) GetAll() ([]models.Product, error) {
	query := `SELECT id, name, price, quantity, threshold FROM products ORDER BY id`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Quantity, &p.Threshold); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *PostgresProductRepository) GetByID(id int) (models.Product, error) {
	query := `SELECT id, name, price, quantity, threshold FROM products WHERE id = $1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var p models.Product
	err := r.db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.Name, &p.Price, &p.Quantity, &p.Threshold)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Product{}, ErrProductNotFound
	}
	return p, err
}

func (r *PostgresProductRepository) Update(p models.Product) (models.Product, error) {
	query := `UPDATE products SET name = $1, price = $2, quantity = $3, threshold = $4, updated_at = $5 WHERE id = $6`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := r.db.ExecContext(ctx, query, p.Name, p.Price, p.Quantity, p.Threshold, p.UpdatedAt, p.ID)
	if err != nil {
		return models.Product{}, err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return models.Product{}, ErrProductNotFound
	}
	return p, nil
}

func (r *PostgresProductRepository) Delete(id int) error {
	query := `DELETE FROM products WHERE id = $1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrProductNotFound
	}
	return nil
}

func (r *PostgresProductRepository) Filter(pf ProductFilter) ([]models.Product, int, error) {

	conditions, args, argIdx := filterConditions(pf)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var totalCount int
	filterClause := conditions
	countQuery := "SELECT COUNT(*) FROM products WHERE 1=1" + filterClause
	row := r.db.QueryRowContext(ctx, countQuery, args...)
	if err := row.Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	query := `SELECT id, name, price, quantity, threshold FROM products WHERE 1=1`
	query += conditions
	query += " ORDER BY id"

	if pf.Limit != nil && *pf.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, pf.Limit)
		argIdx++
	}
	if pf.Offset != nil && *pf.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, pf.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Quantity, &p.Threshold); err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}

	return products, totalCount, nil
}

func filterConditions(pf ProductFilter) (string, []any, int) {
	query := ""
	argIdx := 1
	args := []any{}

	if pf.Name != "" {
		query += fmt.Sprintf(" AND name ILIKE $%d", argIdx)
		args = append(args, "%"+pf.Name+"%")
		argIdx++
	}
	if pf.MinPrice != nil {
		query += fmt.Sprintf(" AND price >= $%d", argIdx)
		args = append(args, pf.MinPrice)
		argIdx++
	}
	if pf.MaxPrice != nil {
		query += fmt.Sprintf(" AND price <= $%d", argIdx)
		args = append(args, pf.MaxPrice)
		argIdx++
	}
	if pf.MinQty != nil {
		query += fmt.Sprintf(" AND quantity >= $%d", argIdx)
		args = append(args, pf.MinQty)
		argIdx++
	}
	if pf.MaxQty != nil {
		query += fmt.Sprintf(" AND quantity <= $%d", argIdx)
		args = append(args, pf.MaxQty)
		argIdx++
	}

	return query, args, argIdx
}

func (r *PostgresProductRepository) AdjustQuantity(productID int, delta int) (models.Product, error) {
	query := `
		UPDATE products
		SET quantity = quantity + $1, updated_at = $2
		WHERE id = $3 AND quantity + $1 >= 0
		RETURNING id, name, price, quantity, threshold, created_at, updated_at
	`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var p models.Product
	err := r.db.QueryRowContext(ctx, query, delta, time.Now().UTC(), productID).
		Scan(&p.ID, &p.Name, &p.Price, &p.Quantity, &p.Threshold, &p.CreatedAt, &p.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return models.Product{}, ErrInvalidQuantityChange
	}
	return p, err
}

func (r *PostgresProductRepository) GetByName(name string) (models.Product, error) {
	query := `SELECT id, name, price, quantity, threshold, created_at, updated_at FROM products WHERE name = $1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var p models.Product
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&p.ID, &p.Name, &p.Price, &p.Quantity, &p.Threshold, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Product{}, ErrProductNotFound
	}
	return p, err
}
