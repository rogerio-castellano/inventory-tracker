package main

import (
	"log"
	"net/http"

	"github.com/rogerio-castellano/inventory-tracker/internal/db"
	httpdelivery "github.com/rogerio-castellano/inventory-tracker/internal/http"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

func main() {
	database, err := db.Connect()
	if err != nil {
		log.Fatal("❌ Could not connect to database:", err)
	}

	httpdelivery.SetProductRepo(repo.NewPostgresProductRepository(database))
	httpdelivery.SetMovementRepo(repo.NewPostgresMovementRepository(database))

	r := httpdelivery.NewRouter()
	log.Println("✅ Server running on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
