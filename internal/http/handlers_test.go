package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpdelivery "github.com/rogerio-castellano/inventory-tracker/internal/http"
)

func TestCreateProductHandler_Valid(t *testing.T) {
	r := httpdelivery.NewRouter()
	body := map[string]any{"name": "Laptop", "price": 1500.0}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
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
