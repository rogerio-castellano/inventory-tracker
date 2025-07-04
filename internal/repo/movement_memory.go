package repo

import (
	"time"

	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

type InMemoryMovementRepository struct {
	movements []models.Movement
}

func (r *InMemoryMovementRepository) AddMovement(productId int, delta int, createdAt time.Time) {
	movement := models.Movement{
		ProductID: productId,
		Delta:     delta,
		CreatedAt: createdAt.Format(time.RFC3339),
	}
	r.movements = append(r.movements, movement)
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

// GetByProductID returns all movements for a specific product, optionally filtered by date range and paginated
func (r *InMemoryMovementRepository) GetByProductID(productID int, since, until *time.Time, limit, offset *int) ([]models.Movement, int, error) {
	var filtered []models.Movement
	for _, m := range r.movements {
		if m.ProductID == productID {
			if (since != nil && m.CreatedAt < since.Format(time.RFC3339)) ||
				(until != nil && m.CreatedAt > until.Format(time.RFC3339)) {
				continue
			}
			filtered = append(filtered, m)
		}
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
