package handlers_integrated_test_suite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	handler "github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

func TestAdjustQuantityHandler(t *testing.T) {
	t.Cleanup(clearAllProducts)

	r := api.NewRouter()
	product := handler.ProductRequest{Name: "InventoryItem", Price: 10.0, Quantity: 10}
	w := createProduct(r, product)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created handler.ProductRequest
	json.NewDecoder(w.Body).Decode(&created)

	t.Run("Increase quantity", func(t *testing.T) {
		adj := handler.QuantityAdjustmentRequest{Delta: 5}
		w := adjustProduct(r, created.Id, adj)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var resp handler.ProductResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Quantity != 15 {
			t.Errorf("expected quantity 15, got %v", resp.Quantity)
		}
	})

	t.Run("Decrease quantity", func(t *testing.T) {
		adj := handler.QuantityAdjustmentRequest{Delta: -3}
		w := adjustProduct(r, created.Id, adj)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.ProductResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Quantity != 12.0 {
			t.Errorf("expected quantity 12, got %v", resp.Quantity)
		}
	})

	t.Run("Too much decrease (underflow)", func(t *testing.T) {
		adj := handler.QuantityAdjustmentRequest{Delta: -100}
		w := adjustProduct(r, created.Id, adj)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict, got %d", w.Code)
		}
	})

	t.Run("Invalid ID", func(t *testing.T) {
		adj := handler.QuantityAdjustmentRequest{Delta: 1}
		body, _ := json.Marshal(adj)
		req := httptest.NewRequest(http.MethodPost, "/products/abc/adjust", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", created.Id), bytes.NewBufferString(`{`))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestAdjustQuantityHandler_AtomicAndConcurrent(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	product := handler.ProductRequest{Name: "ConcurrentItem", Price: 10.0, Quantity: 5}
	w := createProduct(r, product)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created handler.ProductResponse
	json.NewDecoder(w.Body).Decode(&created)

	// ❌ Try deducting more than available
	t.Run("Reject over-deduction", func(t *testing.T) {
		adj := handler.QuantityAdjustmentRequest{Delta: -10}
		w := adjustProduct(r, created.Id, adj)

		if w.Code != http.StatusConflict {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	// ✅ Concurrent increment and decrement
	t.Run("Concurrent adjustments are safe", func(t *testing.T) {
		var wg sync.WaitGroup
		successCount := 0
		totalRequests := 10

		for i := range totalRequests {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				delta := -1
				if i%2 == 0 {
					delta = +1
				}
				adj := handler.QuantityAdjustmentRequest{Delta: delta}
				w := adjustProduct(r, created.Id, adj)

				if w.Code == http.StatusOK {
					successCount++
				}
			}(i)
		}
		wg.Wait()

		getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d", created.Id), nil)
		getW := httptest.NewRecorder()
		r.ServeHTTP(getW, getReq)
		var final handler.ProductResponse
		json.NewDecoder(getW.Body).Decode(&final)
		if final.Quantity < 0 {
			t.Errorf("quantity should not go negative, got %d", final.Quantity)
		}
	})
}

func TestGetMovementsHandler(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	product := handler.ProductRequest{Name: "Box", Price: 50.0, Quantity: 10}
	w := createProduct(r, product)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created handler.ProductResponse
	json.NewDecoder(w.Body).Decode(&created)

	// Adjust quantity twice to generate movement log
	adjust := func(delta int) {
		adj := handler.QuantityAdjustmentRequest{Delta: delta}
		w := adjustProduct(r, created.Id, adj)
		if w.Code != http.StatusOK {
			t.Fatalf("failed to adjust quantity: delta %d", delta)
		}
	}
	adjust(3)
	adjust(-2)

	t.Run("Returns movements", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements", created.Id), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var movementsCollection handler.MovementsSearchResult
		if err := json.NewDecoder(w.Body).Decode(&movementsCollection); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if count := len(movementsCollection.Data); count != 2 {
			t.Errorf("expected 2 movements, got %d", count)
		}
	})

	t.Run("Invalid product ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products/abc/movements", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("No movements for nonexistent product", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products/999999/movements", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 400 Not Found, got %d", w.Code)
		}

		var movements handler.MovementsSearchResult
		json.NewDecoder(w.Body).Decode(&movements)
		if count := movements.Meta.TotalCount; count != 0 {
			t.Errorf("expected 0 movements, got %d", count)
		}
	})
}

func TestGetMovementsHandler_Filtering(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	product := handler.ProductRequest{Name: "FilterBox", Price: 80.0, Quantity: 10}
	w := createProduct(r, product)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created handler.ProductResponse
	json.NewDecoder(w.Body).Decode(&created)

	// First adjustment: backdated (manually insert)
	addMovement(models.Movement{
		ProductID: created.Id,
		Delta:     5,
		CreatedAt: time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339),
	})
	// movement :=
	// Second adjustment: recent
	adj := handler.QuantityAdjustmentRequest{Delta: 2}
	w2 := adjustProduct(r, created.Id, adj)
	if w2.Code != http.StatusOK {
		t.Fatalf("failed to adjust product")
	}

	t.Run("since: only recent movement", func(t *testing.T) {
		since := time.Now().Add(-12 * time.Hour).Format(time.RFC3339)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements?since=%s", created.Id, since), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var movementsCollection handler.MovementsSearchResult
		if err := json.NewDecoder(w.Body).Decode(&movementsCollection); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if count := len(movementsCollection.Data); count != 1 {
			t.Errorf("expected 1 recent movement, got %d", count)
		}
	})

	t.Run("until: only old movement", func(t *testing.T) {
		until := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements?until=%s", created.Id, until), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var movementsCollection handler.MovementsSearchResult
		if err := json.NewDecoder(w.Body).Decode(&movementsCollection); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(movementsCollection.Data) != 1 {
			t.Errorf("expected 1 old movement, got %d", len(movementsCollection.Data))
		}
	})

	t.Run("since + until: full range", func(t *testing.T) {
		since := time.Now().Add(-72 * time.Hour).Format(time.RFC3339)
		until := time.Now().Format(time.RFC3339)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements?since=%s&until=%s", created.Id, since, until), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var movementsCollection handler.MovementsSearchResult
		if err := json.NewDecoder(w.Body).Decode(&movementsCollection); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if count := len(movementsCollection.Data); count != 2 {
			t.Errorf("expected 2 movements, got %d", count)
		}
	})

	t.Run("no match range", func(t *testing.T) {
		since := time.Now().Add(-10 * time.Hour).Format(time.RFC3339)
		until := time.Now().Add(-5 * time.Hour).Format(time.RFC3339)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements?since=%s&until=%s", created.Id, since, until), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var movementsCollection handler.MovementsSearchResult
		if err := json.NewDecoder(w.Body).Decode(&movementsCollection); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if count := len(movementsCollection.Data); count != 0 {
			t.Errorf("expected 0 movements, got %d", count)
		}
	})
}

func TestGetMovementsHandler_Pagination(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	product := handler.ProductRequest{Name: "PagedWidget", Price: 20.0, Quantity: 5}
	w := createProduct(r, product)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created handler.ProductResponse
	json.NewDecoder(w.Body).Decode(&created)

	// Generate 3 movements
	deltas := []int{+1, -1, +2}
	for _, d := range deltas {
		adj := handler.QuantityAdjustmentRequest{Delta: d}
		w := adjustProduct(r, created.Id, adj)
		if w.Code != http.StatusOK {
			t.Fatalf("failed to adjust delta %d", d)
		}
	}

	t.Run("limit=1 returns only one", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements?limit=1", created.Id), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.MovementsSearchResult
		json.NewDecoder(w.Body).Decode(&resp)

		if resp.Meta.TotalCount == 0 {
			t.Error("expected total_count in response")
		}

		if count := len(resp.Data); count != 1 {
			t.Errorf("expected 1 item, got %d", count)
		}
	})

	t.Run("offset=1 skips first", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements?offset=1", created.Id), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.MovementsSearchResult
		json.NewDecoder(w.Body).Decode(&resp)
		if count := len(resp.Data); count != 2 {
			t.Errorf("expected 2 items, got %d", count)
		}
	})

	t.Run("limit + offset combined", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements?limit=1&offset=1", created.Id), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.MovementsSearchResult
		json.NewDecoder(w.Body).Decode(&resp)
		items := resp.Data
		if len(items) != 1 {
			t.Errorf("expected 1 item, got %d", len(items))
		}
	})
}

func TestLowStockAlert(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	product := handler.ProductRequest{
		Name:      "AlertItem",
		Price:     50.0,
		Quantity:  5,
		Threshold: 3,
	}
	w := createProduct(r, product)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create product: %d", w.Code)
	}
	var created handler.ProductResponse
	json.NewDecoder(w.Body).Decode(&created)

	// Adjust to just above threshold (5 → 4) → no alert
	t.Run("No alert above threshold", func(t *testing.T) {
		adj := handler.QuantityAdjustmentRequest{Delta: -1}
		w := adjustProduct(r, created.Id, adj)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		// Use a generic map to decode response dynamically, allowing validation even when "low_stock" may be absent
		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)

		if resp["low_stock"] != nil {
			t.Error("expected no low_stock alert above threshold")
		}
	})

	// Adjust to below threshold (4 → 2) → should trigger alert
	t.Run("Alert triggered below threshold", func(t *testing.T) {
		adj := handler.QuantityAdjustmentRequest{Delta: -2}
		w := adjustProduct(r, created.Id, adj)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.ProductResponse
		json.NewDecoder(w.Body).Decode(&resp)

		if resp.LowStock != true {
			t.Error("expected low_stock alert to be true")
		}
	})
}

func TestExportMovementsHandler(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	product := handler.ProductRequest{Name: "Exportable", Price: 100.0, Quantity: 5}
	w := createProduct(r, product)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created handler.ProductResponse
	json.NewDecoder(w.Body).Decode(&created)

	adj := handler.QuantityAdjustmentRequest{Delta: 3}
	w2 := adjustProduct(r, created.Id, adj)
	if w2.Code != http.StatusOK {
		t.Fatalf("failed to adjust product")
	}

	t.Run("Export as JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements/export?format=json", created.Id), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("expected application/json, got %s", ct)
		}
		if !strings.Contains(w.Body.String(), `"delta"`) {
			t.Errorf("expected JSON content with field 'delta', got: %s", w.Body.String())
		}
	})

	t.Run("Export as CSV", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements/export?format=csv", created.Id), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
			t.Errorf("expected text/csv, got %s", ct)
		}
		if !strings.Contains(w.Body.String(), "product_id,delta") {
			t.Errorf("expected CSV header in response, got: %s", w.Body.String())
		}
	})

	t.Run("Invalid format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements/export?format=pdf", created.Id), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Invalid product ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products/abc/movements/export?format=json", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestExportMovementsHandler_Filtered(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	product := handler.ProductRequest{Name: "FilteredExport", Price: 75.0, Quantity: 8}
	w := createProduct(r, product)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created handler.ProductResponse
	json.NewDecoder(w.Body).Decode(&created)

	// Insert one old movement
	addMovement(models.Movement{
		ProductID: created.Id,
		Delta:     -1,
		CreatedAt: time.Now().Add(-72 * time.Hour).UTC().Format(time.RFC3339),
	})
	// Insert one recent movement via API
	adj := handler.QuantityAdjustmentRequest{Delta: 2}
	w2 := adjustProduct(r, created.Id, adj)
	if w2.Code != http.StatusOK {
		t.Fatalf("failed to add recent movement")
	}

	t.Run("Export recent only as JSON", func(t *testing.T) {
		since := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements/export?format=json&since=%s", created.Id, since), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var items []models.Movement
		if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		if count := len(items); count != 1 {
			t.Errorf("expected 1 recent movement, got %d", count)
		}
	})

	t.Run("Export old only as CSV", func(t *testing.T) {
		until := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements/export?format=csv&until=%s", created.Id, until), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		body := w.Body.String()
		lines := strings.Split(strings.TrimSpace(body), "\n")
		if count := len(lines); count-1 != 1 {
			t.Errorf("expected 1 CSV row of data, got %d rows (incl. header)", count-1)
		}
	})
}
