package http

import (
	"encoding/json"
	"net/http"
	"strings"
)

type ProductRequest struct {
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"name":  req.Name,
		"price": req.Price,
	})
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
