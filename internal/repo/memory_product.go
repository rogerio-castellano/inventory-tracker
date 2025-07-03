package repo

import (
	"errors"

	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

// InMemoryProductRepository is an in-memory implementation of ProductRepository.
type InMemoryProductRepository struct {
	products []models.Product
	nextID   int
}

// NewInMemoryProductRepository creates a new instance of InMemoryProductRepository.
func NewInMemoryProductRepository() *InMemoryProductRepository {
	return &InMemoryProductRepository{
		products: []models.Product{},
		nextID:   1,
	}
}

// Create adds a new product to the repository.
func (r *InMemoryProductRepository) Create(product models.Product) (models.Product, error) {
	product.ID = r.nextID
	r.nextID++
	r.products = append(r.products, product)
	return product, nil
}

// GetAll retrieves all products from the repository.
func (r *InMemoryProductRepository) GetAll() ([]models.Product, error) {
	return r.products, nil
}

// GetByID retrieves a product by its ID.
func (r *InMemoryProductRepository) GetByID(id int) (models.Product, error) {
	for _, p := range r.products {
		if p.ID == id {
			return p, nil
		}
	}
	return models.Product{}, ErrProductNotFound
}

// Update modifies an existing product in the repository.
func (r *InMemoryProductRepository) Update(product models.Product) (models.Product, error) {
	for i, p := range r.products {
		if p.ID == product.ID {
			r.products[i] = product
			return product, nil
		}
	}
	return models.Product{}, ErrProductNotFound
}

// Delete removes a product from the repository by its ID.
func (r *InMemoryProductRepository) Delete(id int) error {
	for i, p := range r.products {
		if p.ID == id {
			r.products = append(r.products[:i], r.products[i+1:]...)
			return nil
		}
	}
	return ErrProductNotFound
}

// ErrProductNotFound is returned when a product is not found in the repository.
var ErrProductNotFound = errors.New("product not found")
