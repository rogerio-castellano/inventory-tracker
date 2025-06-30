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
}

func TestCreateProductHandler_Invalid(t *testing.T) {
	r := httpdelivery.NewRouter()

	tests := []struct {
		name       string
		payload    map[string]any
		expectCode int
	}{
		{
			name:       "Empty name and price",
			payload:    map[string]any{"name": "", "price": 0.0},
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Empty name only",
			payload:    map[string]any{"name": "", "price": 100.0},
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Invalid price only",
			payload:    map[string]any{"name": "Mouse", "price": -5.0},
			expectCode: http.StatusBadRequest,
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
		})
	}
}
