package main

import (
	"log"
	"net/http"

	"github.com/rogerio-castellano/inventory-tracker/internal/db"
	httpRoutes "github.com/rogerio-castellano/inventory-tracker/internal/http"
	routes "github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
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

	routes.SetProductRepo(repo.NewPostgresProductRepository(database))
	routes.SetMovementRepo(repo.NewPostgresMovementRepository(database))
	routes.SetUserRepo(repo.NewPostgresUserRepository(database))
	routes.SetMetricsRepo(repo.NewPostgresMetricsRepository(database))

	r := httpRoutes.NewRouter()
	log.Println("✅ Server running on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
