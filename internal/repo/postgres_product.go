package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type PostgresProductRepository struct {
	db *sql.DB
}

func NewPostgresProductRepository(db *sql.DB) *PostgresProductRepository {
	return &PostgresProductRepository{db: db}
}

func (r *PostgresProductRepository) Create(p Product) (Product, error) {
	query := `INSERT INTO products (name, price) VALUES ($1, $2) RETURNING id`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.db.QueryRowContext(ctx, query, p.Name, p.Price).Scan(&p.ID)
	return p, err
}

func (r *PostgresProductRepository) GetAll() ([]Product, error) {
	query := `SELECT id, name, price FROM products ORDER BY id`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *PostgresProductRepository) GetByID(id int) (Product, error) {
	query := `SELECT id, name, price FROM products WHERE id = $1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var p Product
	err := r.db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.Name, &p.Price)
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, ErrProductNotFound
	}
	return p, err
}

func (r *PostgresProductRepository) Update(p Product) (Product, error) {
	query := `UPDATE products SET name = $1, price = $2 WHERE id = $3`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := r.db.ExecContext(ctx, query, p.Name, p.Price, p.ID)
	if err != nil {
		return Product{}, err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return Product{}, ErrProductNotFound
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
