package repo

import "github.com/rogerio-castellano/inventory-tracker/internal/models"

// ProductRepository defines the interface for product data operations.
type ProductRepository interface {
	Create(product models.Product) (models.Product, error)
	GetAll() ([]models.Product, error)
	GetByID(id int) (models.Product, error)
	Update(product models.Product) (models.Product, error)
	Delete(id int) error
}
