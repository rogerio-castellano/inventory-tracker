package main

import (
	"log"
	"net/http"

	httpdelivery "github.com/rogerio-castellano/inventory-tracker/internal/http"
)

func main() {
	r := httpdelivery.NewRouter()
	log.Println("✅ Server running on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
