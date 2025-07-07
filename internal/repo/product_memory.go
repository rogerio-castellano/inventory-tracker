package repo

import (
	"strings"

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

// Filter implements ProductRepository.
func (r *InMemoryProductRepository) Filter(name string, minPrice, maxPrice *float64, minQty, maxQty, offset, limit *int) ([]models.Product, int, error) {
	var filtered []models.Product

	for _, p := range r.products {
		if name != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(name)) {
			continue
		}
		if minPrice != nil && p.Price < *minPrice {
			continue
		}
		if maxPrice != nil && p.Price > *maxPrice {
			continue
		}
		if minQty != nil && p.Quantity < *minQty {
			continue
		}
		if maxQty != nil && p.Quantity > *maxQty {
			continue
		}
		filtered = append(filtered, p)
	}

	if offset != nil && *offset > len(filtered) {
		return nil, 0, nil // If offset is greater than the number of filtered products, return empty slice
	}

	start := 0
	end := len(filtered)
	if (offset != nil && *offset > 0) || (limit != nil && *limit > 0) {
		if offset != nil && *offset < len(filtered) {
			start = *offset
		}
		if limit != nil && *limit > 0 && start+*limit < len(filtered) {
			end = start + *limit
		}
	}

	return filtered[start:end], len(filtered), nil
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

func (r *InMemoryProductRepository) Clear() {
	r.products = []models.Product{}
}

// AdjustQuantity implements ProductRepository.
func (r *InMemoryProductRepository) AdjustQuantity(productId int, delta int) (models.Product, error) {
	product, _ := r.GetByID(productId)

	if product.Quantity+delta < 0 {
		return models.Product{}, ErrInvalidQuantityChange
	}

	product.Quantity += delta
	for i, p := range r.products {
		if p.ID == productId {
			r.products[i] = product
			return product, nil
		}
	}

	return models.Product{}, ErrProductNotFound
}

func (r *InMemoryProductRepository) GetByName(name string) (models.Product, error) {
	for _, p := range r.products {
		if p.Name == name {
			return p, nil
		}
	}
	return models.Product{}, ErrProductNotFound
}
