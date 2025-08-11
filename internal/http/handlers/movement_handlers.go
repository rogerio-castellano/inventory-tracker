package handlers

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rogerio-castellano/inventory-tracker/internal/repo"
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
	if err := readJSON(w, r, &req); err != nil {
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
	if err := writeJSON(w, http.StatusOK, resp); err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
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
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid product ID", http.StatusBadRequest)
		return
	}

	if _, err := productRepo.GetByID(id); err != nil {
		status := http.StatusInternalServerError
		if err == repo.ErrProductNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, "product not found", status)
		return
	}

	q := r.URL.Query()
	since, err := parseTime(q.Get("since"))
	if err != nil {
		http.Error(w, "invalid since date format", http.StatusBadRequest)
		return
	}
	until, err := parseTime(q.Get("until"))
	if err != nil {
		http.Error(w, "invalid until date format", http.StatusBadRequest)
		return
	}

	limit, err := parseNonNegativeInt(q.Get("limit"))
	if err != nil {
		http.Error(w, "invalid limit format", http.StatusBadRequest)
		return
	}
	offset, err := parseNonNegativeInt(q.Get("offset"))
	if err != nil {
		http.Error(w, "invalid offset format", http.StatusBadRequest)
		return
	}

	movements, total, err := movementRepo.GetByProductID(id, repo.MovementFilter{Since: since, Until: until, Offset: offset, Limit: limit})
	if err != nil {
		log.Printf("failed to retrieve movements for product %d: %v", id, err)
		http.Error(w, "could not retrieve movements", http.StatusInternalServerError)
		return
	}

	resp := MovementsSearchResult{
		Data: make([]MovementResponse, len(movements)),
		Meta: Meta{TotalCount: total},
	}
	for i, m := range movements {
		resp.Data[i] = MovementResponse{
			ID:        m.ID,
			ProductID: m.ProductID,
			Delta:     m.Delta,
			CreatedAt: m.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := writeJSON(w, http.StatusOK, resp); err != nil {
		log.Printf("failed to encode response: %v", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
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
	q := r.URL.Query()
	format := q.Get("format")
	if format != "csv" && format != "json" {
		http.Error(w, "format must be 'csv' or 'json'", http.StatusBadRequest)
		return
	}

	since, err := parseTime(q.Get("since"))
	if err != nil {
		http.Error(w, "invalid since date format", http.StatusBadRequest)
		return
	}
	until, err := parseTime(q.Get("until"))
	if err != nil {
		http.Error(w, "invalid until date format", http.StatusBadRequest)
		return
	}

	movements, _, err := movementRepo.GetByProductID(id, repo.MovementFilter{Since: since, Until: until})
	if err != nil {
		http.Error(w, "could not retrieve movements", http.StatusInternalServerError)
		return
	}

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="movements.json"`)

		if err := writeJSON(w, http.StatusOK, movements); err != nil {
			log.Printf("Failed to write JSON response: %v", err)
		}
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

func parseID(idStr string) (int, error) {
	return strconv.Atoi(idStr)
}

func parseTime(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	raw = fixRFC3339(raw)
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		log.Printf("invalid time: %s", raw)
		return nil, err
	}
	return &t, nil
}

func fixRFC3339(s string) string {
	if len(s) == len(time.RFC3339) && s[len(s)-6] == ' ' {
		return s[:len(s)-6] + "+" + s[len(s)-5:]
	}
	return s
}

func parseNonNegativeInt(s string) (*int, error) {
	if s == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return nil, fmt.Errorf("invalid non-negative int: %s", s)
	}
	return &v, nil
}
