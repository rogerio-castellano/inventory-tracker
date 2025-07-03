package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	models "github.com/rogerio-castellano/inventory-tracker/internal/models"
)

type PostgresProductRepository struct {
	db *sql.DB
}

func NewPostgresProductRepository(db *sql.DB) *PostgresProductRepository {
	return &PostgresProductRepository{db: db}
}

func (r *PostgresProductRepository) Create(p models.Product) (models.Product, error) {
	query := `INSERT INTO products (name, price, created_at, updated_at) VALUES ($1, $2, $3, $4) RETURNING id`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.db.QueryRowContext(ctx, query, p.Name, p.Price, p.CreatedAt, p.UpdatedAt).Scan(&p.ID)
	return p, err
}

func (r *PostgresProductRepository) GetAll() ([]models.Product, error) {
	query := `SELECT id, name, price FROM products ORDER BY id`
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
		if err := rows.Scan(&p.ID, &p.Name, &p.Price); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *PostgresProductRepository) GetByID(id int) (models.Product, error) {
	query := `SELECT id, name, price FROM products WHERE id = $1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var p models.Product
	err := r.db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.Name, &p.Price)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Product{}, ErrProductNotFound
	}
	return p, err
}

func (r *PostgresProductRepository) Update(p models.Product) (models.Product, error) {
	query := `UPDATE products SET name = $1, price = $2, updated_at = $3 WHERE id = $4`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := r.db.ExecContext(ctx, query, p.Name, p.Price, p.UpdatedAt, p.ID)
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
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrProductNotFound
	}
	return nil
}
