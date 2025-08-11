package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rogerio-castellano/inventory-tracker/internal/auth"
	"github.com/rogerio-castellano/inventory-tracker/internal/db"
	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	ban "github.com/rogerio-castellano/inventory-tracker/internal/http/ban"
	"github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
	"github.com/rogerio-castellano/inventory-tracker/internal/redissvc"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

var rdb = redis.NewClient(&redis.Options{
	Addr: "inventory-redis:6379",
})
var ctx = context.Background()

// @title Inventory Tracker API
// @version 1.0
// @description REST API for managing inventory products and stock movements.
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	go auth.StartRefreshTokenCleaner(30 * time.Minute)
	go ban.StartDailyBanSummary(time.Hour * 24)
	go api.StartVisitorCleanupLoop()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	defer rdb.Close()

	redisService := redissvc.NewRedisService(rdb, ctx)
	handlers.SetRedisService(redisService)
	ban.SetRedisService(redisService)
	api.SetRedisService(redisService)

	database, err := db.Connect()
	if err != nil {
		log.Fatal("❌ Could not connect to database:", err)
	}
	defer database.Close()

	handlers.SetProductRepo(repo.NewPostgresProductRepository(database))
	handlers.SetMovementRepo(repo.NewPostgresMovementRepository(database))
	handlers.SetUserRepo(repo.NewPostgresUserRepository(database))
	handlers.SetMetricsRepo(repo.NewPostgresMetricsRepository(database))

	r := api.NewRouter()
	log.Println("✅ Server running on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
