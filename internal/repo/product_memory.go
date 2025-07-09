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

func matchesFilter(p models.Product, pf ProductFilter) bool {
	if pf.Name != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(pf.Name)) {
		return false
	}
	if pf.MinPrice != nil && p.Price < *pf.MinPrice {
		return false
	}
	if pf.MaxPrice != nil && p.Price > *pf.MaxPrice {
		return false
	}
	if pf.MinQty != nil && p.Quantity < *pf.MinQty {
		return false
	}
	if pf.MaxQty != nil && p.Quantity > *pf.MaxQty {
		return false
	}
	return true
}

func (r *InMemoryProductRepository) Filter(pf ProductFilter) ([]models.Product, int, error) {
	var filtered []models.Product

	for _, p := range r.products {
		if matchesFilter(p, pf) {
			filtered = append(filtered, p)
		}
	}

	// If offset is greater than the number of filtered products, return empty slice
	if pf.Offset != nil && *pf.Offset > len(filtered) {
		return []models.Product{}, 0, nil
	}

	start := 0
	if pf.Offset != nil {
		start = clamp(*pf.Offset, 0, len(filtered))
	}

	end := len(filtered)
	if pf.Limit != nil && *pf.Limit > 0 {
		end = clamp(start+*pf.Limit, start, len(filtered))
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
