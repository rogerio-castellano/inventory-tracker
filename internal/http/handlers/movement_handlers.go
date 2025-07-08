package handlers

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

// AdjustQuantityHandler godoc
// @Summary Adjust quantity of a product
// @Tags inventory
// @Accept json
// @Produce json
// @Param id path int true "Product ID"
// @Param adjustment body QuantityAdjustmentRequest true "Quantity change"
// @Success 200 {object} ProductResponse
// @Failure 400 {string} string "Invalid adjustment"
// @Failure 404 {string} string "Not found"
// @Failure 500 {string} string "Internal error"
// @Router /products/{id}/adjust [post]
// @Security BearerAuth
func AdjustQuantityHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

	var req QuantityAdjustmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	product, err := productRepo.AdjustQuantity(id, req.Delta)
	if err != nil {
		if err == repo.ErrInvalidQuantityChange {
			http.Error(w, "quantity cannot be negative", http.StatusConflict)
			return
		}
		http.Error(w, "could not update quantity", http.StatusInternalServerError)
		return
	}
	_ = movementRepo.Log(id, req.Delta)

	if product.Quantity < product.Threshold {
		log.Printf("⚠️ ALERT: Product %d (%s) is below threshold! Qty=%d, Threshold=%d",
			product.ID, product.Name, product.Quantity, product.Threshold)
	}

	resp := ProductResponse{
		Id:        product.ID,
		Name:      product.Name,
		Price:     product.Price,
		Quantity:  product.Quantity,
		Threshold: product.Threshold,
		LowStock:  product.Quantity < product.Threshold,
	}
	if product.Quantity < product.Threshold {
		resp.LowStock = true
	}

	json.NewEncoder(w).Encode(resp)

}

// GetMovementsHandler godoc
// @Summary Get product movement logs
// @Tags movements
// @Produce json
// @Param id path int true "Product ID"
// @Param since query string false "Filter movements from this timestamp (RFC3339)"
// @Param until query string false "Filter movements until this timestamp (RFC3339)"
// @Param offset query int false "Offset for pagination"
// @Param limit query int false "Limit for pagination"
// @Success 200 {object} MovementsSearchResult
// @Failure 400 {string} string "Invalid input"
// @Failure 404 {string} string "Product not found"
// @Failure 500 {string} string "Internal error"
// @Router /products/{id}/movements [get]
func GetMovementsHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

	// Validate the product ID
	_, err = productRepo.GetByID(id)
	if err != nil {
		if err == repo.ErrProductNotFound {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
	}

	sinceStr := r.URL.Query().Get("since")
	untilStr := r.URL.Query().Get("until")

	// Reverse the substitution from + for space in the date parameters, otherwise
	// time.Parse will fail with an error.
	// This is necessary because URL query parameters replace + with a space.
	// Example: 2025-07-03T17:44:03+02:00 becomes 2025-07-03T17:44:03 02:00 on r.URL.Query().Get()
	if len(sinceStr) == len(time.RFC3339) && sinceStr[len(sinceStr)-6] == ' ' {
		sinceStr = sinceStr[:len(sinceStr)-6] + "+" + sinceStr[len(sinceStr)-5:]
	}
	if len(untilStr) == len(time.RFC3339) && untilStr[len(untilStr)-6] == ' ' {
		untilStr = untilStr[:len(untilStr)-6] + "+" + untilStr[len(untilStr)-5:]
	}

	var since, until *time.Time
	if sinceStr != "" {
		if ts, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &ts
		} else {
			log.Printf("could not parse since date %s: %v", sinceStr, err)
			http.Error(w, "invalid since date format", http.StatusBadRequest)
			return
		}
	}
	if untilStr != "" {
		if ts, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = &ts
		} else {
			log.Printf("could not parse until date %s: %v", untilStr, err)
			http.Error(w, "invalid until date format", http.StatusBadRequest)
			return
		}
	}

	var limit, offset *int

	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = &v
		} else {
			log.Printf("could not parse limit %s: %v", limitStr, err)
			http.Error(w, "invalid limit format", http.StatusBadRequest)
			return
		}
	}

	if limit != nil && *limit <= 0 {
		log.Printf("invalid limit %d, must be greater than zero", *limit)
		http.Error(w, "limit must be greater than zero", http.StatusBadRequest)
		return
	}

	offsetStr := r.URL.Query().Get("offset")
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			offset = &v
		} else {
			log.Printf("could not parse offset %s: %v", offsetStr, err)
			http.Error(w, "invalid offset format", http.StatusBadRequest)
			return
		}
	}

	if offset != nil && *offset < 0 {
		log.Printf("invalid offset %d, must be zero or positive", *offset)
		http.Error(w, "offset must be zero or positive", http.StatusBadRequest)
		return
	}

	movements, total, err := movementRepo.GetByProductID(id, since, until, limit, offset)

	if err != nil {
		log.Printf("could not retrieve movements for product %d: %v", id, err)
		http.Error(w, "could not retrieve movements", http.StatusInternalServerError)
		return
	}
	response := MovementsSearchResult{
		Data: make([]MovementResponse, len(movements)),
		Meta: Meta{TotalCount: total},
	}

	for i, m := range movements {
		response.Data[i] = MovementResponse{
			ID:        m.ID,
			ProductID: m.ProductID,
			Delta:     m.Delta,
			CreatedAt: m.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ExportMovementsHandler godoc
// @Summary Export product movement logs
// @Tags movements
// @Produce text/csv, application/json
// @Param id path int true "Product ID"
// @Param format query string true "Export format (csv or json)"
// @Param since query string false "Filter from timestamp (RFC3339)"
// @Param until query string false "Filter until timestamp (RFC3339)"
// @Success 200 {file} file
// @Failure 400 {string} string "Invalid input"
// @Failure 500 {string} string "Internal error"
// @Router /products/{id}/movements/export [get]
func ExportMovementsHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

	format := r.URL.Query().Get("format")
	if format != "csv" && format != "json" {
		http.Error(w, "format must be 'csv' or 'json'", http.StatusBadRequest)
		return
	}

	sinceStr := r.URL.Query().Get("since")
	untilStr := r.URL.Query().Get("until")

	if len(sinceStr) == len(time.RFC3339) && sinceStr[len(sinceStr)-6] == ' ' {
		sinceStr = sinceStr[:len(sinceStr)-6] + "+" + sinceStr[len(sinceStr)-5:]
	}
	if len(untilStr) == len(time.RFC3339) && untilStr[len(untilStr)-6] == ' ' {
		untilStr = untilStr[:len(untilStr)-6] + "+" + untilStr[len(untilStr)-5:]
	}

	var since, until *time.Time
	if sinceStr != "" {
		if ts, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &ts
		}
	}
	if untilStr != "" {
		if ts, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = &ts
		}
	}

	movements, _, err := movementRepo.GetByProductID(id, since, until, nil, nil)
	if err != nil {
		http.Error(w, "could not retrieve movements", http.StatusInternalServerError)
		return
	}

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="movements.json"`)
		json.NewEncoder(w).Encode(movements)

	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="movements.csv"`)

		csvWriter := csv.NewWriter(w)
		_ = csvWriter.Write([]string{"id", "product_id", "delta", "c"})
		for _, m := range movements {
			_ = csvWriter.Write([]string{
				strconv.Itoa(m.ID),
				strconv.Itoa(m.ProductID),
				strconv.Itoa(m.Delta),
				m.CreatedAt,
			})
		}
		csvWriter.Flush()
	}
}
