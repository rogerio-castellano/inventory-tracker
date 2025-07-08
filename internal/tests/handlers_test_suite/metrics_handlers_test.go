package handlers_test_suite

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	handler "github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
	"github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

func TestDashboardMetricsHandler(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	// Create 3 products (2 below threshold)
	products := []handler.ProductRequest{
		{Name: "Keyboard", Price: 40.0, Quantity: 10, Threshold: 5},
		{Name: "Mouse", Price: 20.0, Quantity: 1, Threshold: 5},    // below threshold
		{Name: "Monitor", Price: 150.0, Quantity: 2, Threshold: 3}, // below threshold
	}
	var mouseID int
	for _, p := range products {
		w := createProduct(r, p)
		if w.Code != http.StatusCreated {
			t.Fatalf("product creation failed: %d", w.Code)
		}
		if p.Name == "Mouse" {
			var resp handler.ProductResponse
			json.NewDecoder(w.Body).Decode(&resp)
			mouseID = resp.Id
		}
	}

	// Add 3 movements for Mouse
	for range 3 {
		adj := handler.QuantityAdjustmentRequest{Delta: 1}
		w := adjustProduct(r, mouseID, adj)

		if w.Code != http.StatusOK {
			t.Fatalf("failed to adjust quantity: %d", w.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var metrics repo.Metrics
	if err := json.NewDecoder(w.Body).Decode(&metrics); err != nil {
		t.Fatalf("failed to decode metrics: %v", err)
	}

	if metrics.TotalProducts != 3 {
		t.Errorf("expected 3 products, got %v", metrics.TotalProducts)
	}
	if metrics.TotalMovements < 3 {
		t.Errorf("expected at least 3 movements, got %v", metrics.TotalMovements)
	}
	if metrics.LowStockCount != 2 {
		t.Errorf("expected 2 low stock products, got %v", metrics.LowStockCount)
	}

	mp := metrics.MostMovedProduct
	if mp.Name != "Mouse" {
		t.Errorf("expected Mouse as most moved, got %v", mp.Name)
	}
	if mp.MovementCount != 3 {
		t.Errorf("expected 3 movements, got %v", mp.MovementCount)
	}
}

func TestDashboardMetricsHandler_Enhanced(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	products := []handler.ProductRequest{
		{Name: "Keyboard", Price: 50.0, Quantity: 5, Threshold: 2},
		{Name: "Mouse", Price: 25.0, Quantity: 2, Threshold: 3},    // low stock
		{Name: "Monitor", Price: 200.0, Quantity: 1, Threshold: 2}, // low stock
	}
	var mouseID int
	for _, p := range products {
		w := createProduct(r, p)
		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create product: %d", w.Code)
		}
		if p.Name == "Mouse" {
			var resp handler.ProductResponse
			json.NewDecoder(w.Body).Decode(&resp)
			mouseID = resp.Id
		}
	}

	// Add 5 movements for Mouse
	for range 5 {
		adj := handler.QuantityAdjustmentRequest{Delta: 1}
		adjustProduct(r, mouseID, adj)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var m repo.Metrics
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if m.TotalProducts != 3 {
		t.Errorf("expected total_products name to be 3, got %v", m.TotalProducts)
	}

	if m.TotalMovements != 5 {
		t.Errorf("expected total_movements name to be 5, got %v", m.TotalMovements)
	}

	if m.LowStockCount != 1 {
		t.Errorf("expected low_stock_count name to be 1, got %v", m.LowStockCount)
	}

	wantAveragePrice := float64(50+25+200) / 3.0
	if m.AveragePrice != wantAveragePrice {
		t.Errorf("expected average_price name to be %v = 275, got %v", wantAveragePrice, m.AveragePrice)
	}

	wantTotalStockValue := float64(50*5 + 25*(2+5) + 200*1)
	if m.TotalStockValue != wantTotalStockValue {
		t.Errorf("expected total_stock_value name to be %v = , got %v", wantTotalStockValue, m.TotalStockValue)
	}

	wantTotalQuantity := 5 + (2 + 5) + 1
	if m.TotalQuantity != wantTotalQuantity {
		t.Errorf("expected total_quantity name to be %v, got %v", wantTotalQuantity, m.TotalQuantity)
	}

	mp := m.MostMovedProduct
	if mp.Name != "Mouse" {
		t.Errorf("expected most_moved_product name to be Mouse, got %v", mp.Name)
	}
	if mp.MovementCount != 5 {
		t.Errorf("expected Mouse movement_count = 5, got %v", mp.MovementCount)
	}

	tops := m.Top5Movers
	if len(tops) == 0 {
		t.Fatal("expected at least 1 top mover")
	}
	first := tops[0]
	if first.Name != "Mouse" || first.Count != 5 {
		t.Errorf("expected Mouse as top mover with count 5, got %v", first)
	}
}
