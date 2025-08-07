package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

// GetDashboardMetricsHandler godoc
// @Summary Dashboard metrics for admin view
// @Tags metrics
// @Produce json
// @Success 200 {object} repo.Metrics
// @Failure 500 {string} string "Internal error"
// @Router /metrics/dashboard [get]
func GetDashboardMetricsHandler(w http.ResponseWriter, r *http.Request) {
	m, err := metricsRepo.GetDashboardMetrics()
	if err != nil {
		http.Error(w, "failed to fetch metrics", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(m); err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}
