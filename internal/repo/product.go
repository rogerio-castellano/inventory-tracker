package repo

import "errors"

// Product represents a product entity in the inventory system.
type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// ProductRepository defines the interface for product data operations.
type ProductRepository interface {
	Create(product Product) (Product, error)
	GetAll() ([]Product, error)
	GetByID(id int) (Product, error)
	Update(product Product) (Product, error)
	Delete(id int) error
}

// InMemoryProductRepository is an in-memory implementation of ProductRepository.
type InMemoryProductRepository struct {
	products []Product
	nextID   int
}

// NewInMemoryProductRepository creates a new instance of InMemoryProductRepository.
func NewInMemoryProductRepository() *InMemoryProductRepository {
	return &InMemoryProductRepository{
		products: []Product{},
		nextID:   1,
	}
}

// Create adds a new product to the repository.
func (r *InMemoryProductRepository) Create(product Product) (Product, error) {
	product.ID = r.nextID
	r.nextID++
	r.products = append(r.products, product)
	return product, nil
}

// GetAll retrieves all products from the repository.
func (r *InMemoryProductRepository) GetAll() ([]Product, error) {
	return r.products, nil
}

// GetByID retrieves a product by its ID.
func (r *InMemoryProductRepository) GetByID(id int) (Product, error) {
	for _, p := range r.products {
		if p.ID == id {
			return p, nil
		}
	}
	return Product{}, ErrProductNotFound
}

// Update modifies an existing product in the repository.
func (r *InMemoryProductRepository) Update(product Product) (Product, error) {
	for i, p := range r.products {
		if p.ID == product.ID {
			r.products[i] = product
			return product, nil
		}
	}
	return Product{}, ErrProductNotFound
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
