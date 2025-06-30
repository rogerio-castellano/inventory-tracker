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
	body := map[string]any{"name": "", "price": 0.0}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}
}
