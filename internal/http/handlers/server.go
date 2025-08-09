package handlers

import (
	"github.com/rogerio-castellano/inventory-tracker/internal/auth"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

var (
	productRepo  repo.ProductRepository
	movementRepo repo.MovementRepository
	metricsRepo  repo.MetricsRepository
	userRepo     repo.UserRepository
	AuthSvc      *auth.AuthService
)

func SetProductRepo(r repo.ProductRepository) {
	productRepo = r
}

func SetMovementRepo(r repo.MovementRepository) {
	movementRepo = r
}

func SetMetricsRepo(r repo.MetricsRepository) {
	metricsRepo = r
}

func SetUserRepo(r repo.UserRepository) {
	userRepo = r
}

func SetAuthService(a *auth.AuthService) {
	AuthSvc = a
}
