package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	models "github.com/rogerio-castellano/inventory-tracker/internal/models"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

// CreateProductHandler godoc
// @Summary Create a new product
// @Description Adds a product to the inventory
// @Tags products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param product body ProductRequest true "Product to add"
// @Success 201 {object} ProductResponse
// @Failure 400 {object} map[string]string
// @Router /products [post]
func CreateProductHandler(w http.ResponseWriter, r *http.Request) {
	var req ProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	validationErrors := validateProduct(req)
	if len(validationErrors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(validationErrors)
		return
	}

	product := models.Product{
		Name:      req.Name,
		Price:     req.Price,
		Quantity:  req.Quantity,
		Threshold: req.Threshold,
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	created, err := productRepo.Create(product)
	if err != nil {
		if errors.Is(err, repo.ErrDuplicatedValueUnique) {
			http.Error(w, "could not create product: product name duplicated", http.StatusInternalServerError)
			return
		}
		http.Error(w, "could not create product", http.StatusInternalServerError)
		return
	}

	resp := ProductResponse{
		Id:        created.ID,
		Name:      created.Name,
		Price:     created.Price,
		Quantity:  created.Quantity,
		Threshold: created.Threshold,
		LowStock:  created.Quantity < created.Threshold,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// GetProductsHandler godoc
// @Summary List all products
// @Tags products
// @Produce json
// @Success 200 {array} ProductResponse
// @Failure 500 {string} string "Internal error"
// @Router /products [get]
func GetProductsHandler(w http.ResponseWriter, r *http.Request) {
	products, err := productRepo.GetAll()
	if err != nil {
		http.Error(w, "could not fetch products", http.StatusInternalServerError)
		return
	}
	response := make([]ProductResponse, len(products))
	for i, p := range products {
		response[i] = ProductResponse{
			Id:        p.ID,
			Name:      p.Name,
			Price:     p.Price,
			Quantity:  p.Quantity,
			Threshold: p.Threshold,
			LowStock:  p.Quantity < p.Threshold,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetProductByIDHandler godoc
// @Summary Get product by ID
// @Tags products
// @Produce json
// @Param id path int true "Product ID"
// @Success 200 {object} ProductResponse
// @Failure 400 {string} string "Invalid ID"
// @Failure 404 {string} string "Not found"
// @Failure 500 {string} string "Internal error"
// @Router /products/{id} [get]
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
		Id:        product.ID,
		Name:      product.Name,
		Price:     product.Price,
		Quantity:  product.Quantity,
		Threshold: product.Threshold,
		LowStock:  product.Quantity < product.Threshold,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// DeleteProductHandler godoc
// @Summary Delete a product
// @Tags products
// @Param id path int true "Product ID"
// @Success 204 "Deleted successfully"
// @Failure 400 {string} string "Invalid ID"
// @Failure 404 {string} string "Not found"
// @Failure 500 {string} string "Internal error"
// @Router /products/{id} [delete]
// @Security BearerAuth
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

// UpdateProductHandler godoc
// @Summary Update a product
// @Tags products
// @Accept json
// @Produce json
// @Param id path int true "Product ID"
// @Param product body ProductRequest true "Updated product"
// @Success 200 {object} ProductResponse
// @Failure 400 {object} map[string]any
// @Failure 404 {string} string "Not found"
// @Failure 500 {string} string "Internal error"
// @Router /products/{id} [put]
// @Security BearerAuth
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
		json.NewEncoder(w).Encode(validationErrors)
		return
	}

	product := models.Product{
		ID:        id,
		Name:      req.Name,
		Price:     req.Price,
		Quantity:  req.Quantity,
		Threshold: req.Threshold,
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
		Id:        updated.ID,
		Name:      updated.Name,
		Price:     updated.Price,
		Quantity:  updated.Quantity,
		Threshold: updated.Threshold,
		LowStock:  updated.Quantity < updated.Threshold,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func parseFloatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func parseIntPtr(s string) *int {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

// FilterProductsHandler godoc
// @Summary Filter and paginate products
// @Tags products
// @Produce json
// @Param name query string false "Filter by name"
// @Param minPrice query number false "Minimum price"
// @Param maxPrice query number false "Maximum price"
// @Param minQty query int false "Minimum quantity"
// @Param maxQty query int false "Maximum quantity"
// @Param offset query int false "Offset for pagination"
// @Param limit query int false "Limit for pagination"
// @Success 200 {object} ProductsSearchResult
// @Failure 400 {string} string "Invalid query"
// @Failure 500 {string} string "Internal error"
// @Router /products/search [get]
func FilterProductsHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := repo.ProductFilter{
		Name:     q.Get("name"),
		MinPrice: parseFloatPtr(q.Get("minPrice")),
		MaxPrice: parseFloatPtr(q.Get("maxPrice")),
		MinQty:   parseIntPtr(q.Get("minQty")),
		MaxQty:   parseIntPtr(q.Get("maxQty")),
		Offset:   parseIntPtr(q.Get("offset")),
		Limit:    parseIntPtr(q.Get("limit")),
	}

	if filter.Limit != nil && *filter.Limit <= 0 {
		http.Error(w, "limit must be greater than zero", http.StatusBadRequest)
		return
	}
	if filter.Offset != nil && *filter.Offset < 0 {
		http.Error(w, "offset must be zero or positive", http.StatusBadRequest)
		return
	}

	products, total, err := productRepo.Filter(filter)
	if err != nil {
		http.Error(w, "could not filter products", http.StatusInternalServerError)
		return
	}

	resp := ProductsSearchResult{
		Data: make([]ProductResponse, len(products)),
		Meta: Meta{TotalCount: total},
	}
	for i, p := range products {
		resp.Data[i] = ProductResponse{
			Id:        p.ID,
			Name:      p.Name,
			Price:     p.Price,
			Quantity:  p.Quantity,
			Threshold: p.Threshold,
			LowStock:  p.Quantity < p.Threshold,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("failed to encode response: %v", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
