package main

import (
	"log"
	"net/http"

	"github.com/rogerio-castellano/inventory-tracker/internal/db"
	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

// @title Inventory Tracker API
// @version 1.0
// @description REST API for managing inventory products and stock movements.
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	database, err := db.Connect()
	if err != nil {
		log.Fatal("❌ Could not connect to database:", err)
	}

	api.SetProductRepo(repo.NewPostgresProductRepository(database))
	api.SetMovementRepo(repo.NewPostgresMovementRepository(database))
	api.SetUserRepo(repo.NewPostgresUserRepository(database))
	api.SetMetricsRepo(repo.NewPostgresMetricsRepository(database))

	r := api.NewRouter()
	log.Println("✅ Server running on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
