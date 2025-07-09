package repo

import (
	"errors"

	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

// ProductRepository defines the interface for product data operations.
type ProductRepository interface {
	Create(product models.Product) (models.Product, error)
	GetAll() ([]models.Product, error)
	GetByID(id int) (models.Product, error)
	Update(product models.Product) (models.Product, error)
	Delete(id int) error
	Filter(pf ProductFilter) ([]models.Product, int, error)
	AdjustQuantity(productId int, delta int) (models.Product, error)
	GetByName(name string) (models.Product, error)
}

var ErrInvalidQuantityChange = errors.New("insufficient quantity or product not found")
var ErrProductNotFound = errors.New("product not found")

func clamp(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
