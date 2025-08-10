package handlers

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/rogerio-castellano/inventory-tracker/internal/redissvc"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

var (
	productRepo  repo.ProductRepository
	movementRepo repo.MovementRepository
	metricsRepo  repo.MetricsRepository
	userRepo     repo.UserRepository

	Rdb *redis.Client
	Ctx context.Context
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

func SetRedisService(rs *redissvc.RedisService) {
	Rdb = rs.Rdb()
	Ctx = rs.Ctx()
}
