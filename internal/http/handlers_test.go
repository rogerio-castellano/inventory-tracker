package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	httpdelivery "github.com/rogerio-castellano/inventory-tracker/internal/http"
)

func TestCreateProductHandler_Valid(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := httpdelivery.NewRouter()
	body := map[string]any{"name": "Laptop", "price": 1500.0}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if resp["name"] != "Laptop" {
		t.Errorf("expected name 'Laptop', got %v", resp["name"])
	}
	if resp["price"] != 1500.0 {
		t.Errorf("expected price 1500.0, got %v", resp["price"])
	}
}

func TestCreateProductHandler_Invalid(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := httpdelivery.NewRouter()

	tests := []struct {
		name           string
		payload        map[string]any
		expectCode     int
		expectedErrors []string
	}{
		{
			name:           "Empty name and price",
			payload:        map[string]any{"name": "", "price": 0.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"name", "price"},
		},
		{
			name:           "Empty name only",
			payload:        map[string]any{"name": "", "price": 100.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"name"},
		},
		{
			name:           "Invalid price only",
			payload:        map[string]any{"name": "Mouse", "price": -5.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"price"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBody, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonBody))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.expectCode {
				t.Errorf("expected status %d, got %d", tt.expectCode, w.Code)
			}

			var resp map[string]map[string]string
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("error decoding response: %v", err)
			}

			errorsMap := resp["errors"]
			for _, field := range tt.expectedErrors {
				if _, ok := errorsMap[field]; !ok {
					t.Errorf("expected error for field %q, but not found", field)
				}
			}
		})
	}
}

func TestCreateProductHandler_MalformedJSON(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := httpdelivery.NewRouter()
	badJSON := `{"name": "Invalid" "price": 100}` // missing comma
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(badJSON))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request, got %d", w.Code)
	}

	expectedBody := "invalid input\n"
	if w.Body.String() != expectedBody {
		t.Errorf("expected response body %q, got %q", expectedBody, w.Body.String())
	}
}

func TestGetProductsHandler(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := httpdelivery.NewRouter()

	// Create products to ensure we have something to retrieve
	createBody := map[string]any{"name": "Phone", "price": 999.99}
	jsonCreateBody, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreateBody))
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for product creation, got %d", createW.Code)
	}
	// Create a second product
	createBody2 := map[string]any{"name": "Tablet", "price": 499.99}
	jsonCreateBody2, _ := json.Marshal(createBody2)
	createReq2 := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreateBody2))
	createW2 := httptest.NewRecorder()
	r.ServeHTTP(createW2, createReq2)
	if createW2.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for second product creation, got %d", createW2.Code)
	}

	// Now retrieve the products
	getReq := httptest.NewRequest(http.MethodGet, "/products", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for product retrieval, got %d", getW.Code)
	}

	var products []map[string]any
	if err := json.NewDecoder(getW.Body).Decode(&products); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}

	if len(products) == 0 {
		t.Error("expected at least one product, got none")
	}
	if products[0]["name"] != "Phone" {
		t.Errorf("expected product name 'Phone', got %v", products[0]["name"])
	}
	if products[0]["price"] != 999.99 {
		t.Errorf("expected product price 999.99, got %v", products[0]["price"])
	}

	// Check the second product as well
	if len(products) < 2 {
		t.Errorf("expected at least two products, got %d", len(products))
	} else {
		if products[1]["name"] != "Tablet" {
			t.Errorf("expected product name 'Tablet', got %v", products[1]["name"])
		}
		if products[1]["price"] != 499.99 {
			t.Errorf("expected product price 499.99, got %v", products[1]["price"])
		}
	}
}

// clearAllProducts removes all products using the HTTP API endpoints.
func clearAllProducts() {
	r := httpdelivery.NewRouter()
	getReq := httptest.NewRequest(http.MethodGet, "/products", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		return // nothing to clear or error
	}
	var products []map[string]any
	if err := json.NewDecoder(getW.Body).Decode(&products); err != nil {
		return
	}
	for _, p := range products {
		id := fmt.Sprintf("%v", p["id"])
		deleteReq := httptest.NewRequest(http.MethodDelete, "/products/"+id, nil)
		deleteW := httptest.NewRecorder()
		r.ServeHTTP(deleteW, deleteReq)
	}
}
