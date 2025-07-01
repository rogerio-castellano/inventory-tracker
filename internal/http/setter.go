package http

import "github.com/rogerio-castellano/inventory-tracker/internal/repo"

var productRepo repo.ProductRepository

func SetProductRepo(r repo.ProductRepository) {
	productRepo = r
}
