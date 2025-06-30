package http

import (
	"encoding/json"
	"net/http"
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
	// For now, just echo the product back
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"name":  req.Name,
		"price": req.Price,
	})
}
