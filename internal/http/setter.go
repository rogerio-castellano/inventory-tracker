package http

import "github.com/rogerio-castellano/inventory-tracker/internal/repo"

var productRepo repo.ProductRepository
var movementRepo repo.MovementRepository

func SetProductRepo(r repo.ProductRepository) {
	productRepo = r
}

func SetMovementRepo(r repo.MovementRepository) {
	movementRepo = r
}
