package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

type ProductRequest struct {
	Id    int     `json:"id,omitempty"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type ProductResponse struct {
	Id    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func CreateProductHandler(w http.ResponseWriter, r *http.Request) {
	var req ProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	validationErrors := validateProduct(req)
	if len(validationErrors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"errors": validationErrors,
		})
		return
	}

	product := repo.Product{
		Name:  req.Name,
		Price: req.Price,
	}
	created, err := productRepo.Create(product)
	if err != nil {
		http.Error(w, "could not create product", http.StatusInternalServerError)
		return
	}

	resp := ProductResponse{
		Id:    created.ID,
		Name:  created.Name,
		Price: created.Price,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func GetProductsHandler(w http.ResponseWriter, r *http.Request) {
	products, err := productRepo.GetAll()
	if err != nil {
		http.Error(w, "could not fetch products", http.StatusInternalServerError)
		return
	}
	responses := make([]ProductResponse, len(products))
	for i, p := range products {
		responses[i] = ProductResponse{
			Id:    p.ID,
			Name:  p.Name,
			Price: p.Price,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func GetProductByIDHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

	product, err := productRepo.GetByID(id)
	if err != nil {
		if err == repo.ErrProductNotFound {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "could not fetch product", http.StatusInternalServerError)
		return
	}
	resp := ProductResponse{
		Id:    product.ID,
		Name:  product.Name,
		Price: product.Price,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func DeleteProductHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id") // Use chi to get the path parameter
	if idStr == "" {
		http.Error(w, "product ID is required", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}
	if err := productRepo.Delete(id); err != nil {
		if err == repo.ErrProductNotFound {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "could not delete product", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateProduct(p ProductRequest) map[string]string {
	errs := make(map[string]string)
	if strings.TrimSpace(p.Name) == "" {
		errs["name"] = "Name is required"
	}
	if p.Price <= 0 {
		errs["price"] = "Price must be greater than zero"
	}
	return errs
}
