package repo

import "github.com/rogerio-castellano/inventory-tracker/internal/models"

type MovementRepository interface {
	Log(productID, delta int) error
	GetByProductID(productID int) ([]models.Movement, error)
}
