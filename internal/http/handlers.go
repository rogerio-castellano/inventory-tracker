package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	models "github.com/rogerio-castellano/inventory-tracker/internal/models"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

type ProductRequest struct {
	Id       int     `json:"id,omitempty"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

type ProductResponse struct {
	Id       int     `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

type ProductFilterResponse struct {
	Products   []ProductResponse `json:"products"`
	TotalCount int               `json:"total_count"`
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

	product := models.Product{
		Name:      req.Name,
		Price:     req.Price,
		Quantity:  req.Quantity,
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	created, err := productRepo.Create(product)
	if err != nil {
		http.Error(w, "could not create product", http.StatusInternalServerError)
		return
	}

	resp := ProductResponse{
		Id:       created.ID,
		Name:     created.Name,
		Price:    created.Price,
		Quantity: created.Quantity,
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
			Id:       p.ID,
			Name:     p.Name,
			Price:    p.Price,
			Quantity: p.Quantity,
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
		Id:       product.ID,
		Name:     product.Name,
		Price:    product.Price,
		Quantity: product.Quantity,
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

func UpdateProductHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

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

	product := models.Product{
		ID:        id,
		Name:      req.Name,
		Price:     req.Price,
		Quantity:  req.Quantity,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	updated, err := productRepo.Update(product)
	if err != nil {
		if err == repo.ErrProductNotFound {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "could not update product", http.StatusInternalServerError)
		return
	}

	resp := ProductResponse{
		Id:       updated.ID,
		Name:     updated.Name,
		Price:    updated.Price,
		Quantity: updated.Quantity,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func FilterProductsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	name := query.Get("name")

	var (
		minPrice, maxPrice            *float64
		minQty, maxQty, offset, limit *int
	)

	if v := query.Get("minPrice"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			minPrice = &val
		}
	}
	if v := query.Get("maxPrice"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			maxPrice = &val
		}
	}
	if v := query.Get("minQty"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			minQty = &val
		}
	}
	if v := query.Get("maxQty"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			maxQty = &val
		}
	}

	if v := query.Get("offset"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			offset = &val
		}
	}

	if v := query.Get("limit"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			limit = &val
		}
	}

	if v := query.Get("limit"); v != "" {
		if val, err := strconv.Atoi(v); err != nil || val <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		} else {
			limit = &val
		}
	}

	if v := query.Get("offset"); v != "" {
		if val, err := strconv.Atoi(v); err != nil || val < 0 {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		} else {
			offset = &val
		}
	}

	products, totalCount, err := productRepo.Filter(name, minPrice, maxPrice, minQty, maxQty, offset, limit)
	if err != nil {
		http.Error(w, "could not filter products", http.StatusInternalServerError)
		return
	}

	var response ProductFilterResponse
	for _, p := range products {
		response.Products = append(response.Products, ProductResponse{
			Id:       p.ID,
			Name:     p.Name,
			Price:    p.Price,
			Quantity: p.Quantity,
		})
	}

	response.TotalCount = totalCount

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func validateProduct(p ProductRequest) map[string]string {
	errs := make(map[string]string)
	if strings.TrimSpace(p.Name) == "" {
		errs["name"] = "Name is required"
	}
	if p.Price <= 0 {
		errs["price"] = "Price must be greater than zero"
	}
	if p.Quantity < 0 {
		errs["quantity"] = "Quantity cannot be negative"
	}
	return errs
}
