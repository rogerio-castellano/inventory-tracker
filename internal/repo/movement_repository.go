package repo

import (
	"time"

	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

type MovementRepository interface {
	Log(productID, delta int) error
	GetByProductID(productID int, since, until *time.Time, limit, offset *int) ([]models.Movement, int, error)
}
