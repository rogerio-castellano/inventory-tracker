package repo

import (
	"time"

	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

type InMemoryMovementRepository struct {
	movements []models.Movement
}

func NewInMemoryMovementRepository() *InMemoryMovementRepository {
	return &InMemoryMovementRepository{
		movements: []models.Movement{},
	}
}

// Log inserts a new inventory movement
func (r *InMemoryMovementRepository) Log(productID, delta int) error {
	movement := models.Movement{
		ID:        len(r.movements) + 1,
		ProductID: productID,
		Delta:     delta,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	r.movements = append(r.movements, movement)
	return nil
}

// GetByProductID returns all movements for a specific product
func (r *InMemoryMovementRepository) GetByProductID(productID int) ([]models.Movement, error) {
	var result []models.Movement
	for _, m := range r.movements {
		if m.ProductID == productID {
			result = append(result, m)
		}
	}
	return result, nil
}
