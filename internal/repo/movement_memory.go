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
func (r *InMemoryMovementRepository) GetByProductID(productID int, mf MovementFilter) ([]models.Movement, int, error) {
	var filtered []models.Movement
	for _, m := range r.movements {
		if m.ProductID == productID {
			if (mf.Since != nil && m.CreatedAt < mf.Since.Format(time.RFC3339)) ||
				(mf.Until != nil && m.CreatedAt > mf.Until.Format(time.RFC3339)) {
				continue
			}
			filtered = append(filtered, m)
		}
	}

	if mf.Offset != nil && *mf.Offset > len(filtered) {
		return nil, 0, nil // If offset is greater than the number of filtered products, return empty slice
	}

	start := 0
	if mf.Offset != nil {
		start = clamp(*mf.Offset, 0, len(filtered))
	}

	end := len(filtered)
	if mf.Limit != nil && *mf.Limit > 0 {
		end = clamp(start+*mf.Limit, start, len(filtered))
	}

	return filtered[start:end], len(filtered), nil
}
