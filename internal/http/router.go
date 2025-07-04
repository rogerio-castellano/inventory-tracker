package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Post("/products", CreateProductHandler)
	r.Get("/products", GetProductsHandler)
	r.Get("/products/{id}", GetProductByIDHandler)
	r.Delete("/products/{id}", DeleteProductHandler)
	r.Put("/products/{id}", UpdateProductHandler)
	r.Get("/products/filter", FilterProductsHandler)
	r.Post("/products/{id}/adjust", AdjustQuantityHandler)

	r.Get("/products/{id}/movements", GetMovementsHandler)
	r.Get("/products/{id}/movements/export", ExportMovementsHandler)

	return r
}
