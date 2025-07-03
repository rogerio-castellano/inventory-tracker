package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	httpdelivery "github.com/rogerio-castellano/inventory-tracker/internal/http"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

var testCreatedProductIDs []int

func init() {
	setupTestRepo()
}

func setupTestRepo() {
	httpdelivery.SetProductRepo(repo.NewInMemoryProductRepository())
}

func TestCreateProductHandler_Valid(t *testing.T) {
	t.Cleanup(cleanupCreatedProducts)
	r := httpdelivery.NewRouter()
	body := httpdelivery.ProductRequest{Name: "Laptop", Price: 1500.0, Quantity: 1}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}

	var resp httpdelivery.ProductResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}

	// Store the created product ID for cleanup
	testCreatedProductIDs = append(testCreatedProductIDs, resp.Id)

	if resp.Name != "Laptop" {
		t.Errorf("expected name 'Laptop', got %v", resp.Name)
	}
	if resp.Price != 1500.0 {
		t.Errorf("expected price 1500.0, got %v", resp.Price)
	}
	if resp.Quantity != 1 {
		t.Errorf("expected quantity 1, got %v", resp.Quantity)
	}
}

func TestCreateProductHandler_Invalid(t *testing.T) {
	t.Cleanup(cleanupCreatedProducts)
	r := httpdelivery.NewRouter()

	tests := []struct {
		name           string
		payload        httpdelivery.ProductRequest
		expectCode     int
		expectedErrors []string
	}{
		{
			name:           "Empty name and price",
			payload:        httpdelivery.ProductRequest{Name: "", Price: 0.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"name", "price"},
		},
		{
			name:           "Empty name only",
			payload:        httpdelivery.ProductRequest{Name: "", Price: 100.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"name"},
		},
		{
			name:           "Invalid price only",
			payload:        httpdelivery.ProductRequest{Name: "Mouse", Price: -5.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"price"},
		},
		{
			name:           "Negative quantity",
			payload:        httpdelivery.ProductRequest{Name: "Keyboard", Price: 50.0, Quantity: -1},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"quantity"},
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
	t.Cleanup(cleanupCreatedProducts)
	r := httpdelivery.NewRouter()
	badJSON := `{Name: "Invalid" Price: 100 "}` // missing comma
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
	t.Cleanup(cleanupCreatedProducts)
	r := httpdelivery.NewRouter()

	// Create products to ensure we have something to retrieve
	createBody := httpdelivery.ProductRequest{Name: "Phone", Price: 999.99, Quantity: 1}
	jsonCreateBody, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreateBody))
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for product creation, got %d", createW.Code)
	}
	// Create a second product
	createBody2 := httpdelivery.ProductRequest{Name: "Tablet", Price: 499.99, Quantity: 2}
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

	var products []httpdelivery.ProductResponse
	if err := json.NewDecoder(getW.Body).Decode(&products); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}

	if len(products) == 0 {
		t.Error("expected at least one product, got none")
	}
	if products[0].Name != "Phone" {
		t.Errorf("expected product name 'Phone', got %v", products[0].Name)
	}
	if products[0].Price != 999.99 {
		t.Errorf("expected product price 999.99, got %v", products[0].Price)
	}
	if products[0].Quantity != 1 {
		t.Errorf("expected product quantity 1, got %v", products[0].Quantity)
	}

	// Check the second product as well
	if len(products) < 2 {
		t.Errorf("expected at least two products, got %d", len(products))
	} else {
		if products[1].Name != "Tablet" {
			t.Errorf("expected product name 'Tablet', got %v", products[1].Name)
		}
		if products[1].Price != 499.99 {
			t.Errorf("expected product price 499.99, got %v", products[1].Price)
		}
		if products[1].Quantity != 2 {
			t.Errorf("expected product quantity 2, got %v", products[1].Quantity)
		}
	}
}

func TestUpdateProductHandler_Valid(t *testing.T) {
	t.Cleanup(cleanupCreatedProducts)
	r := httpdelivery.NewRouter()

	// First, create a product
	createBody := httpdelivery.ProductRequest{Name: "Old Name", Price: 100.0, Quantity: 1}
	jsonCreateBody, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreateBody))
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", createW.Code)
	}

	var created httpdelivery.ProductResponse
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("error decoding create response: %v", err)
	}

	// Store the created product ID for cleanup
	testCreatedProductIDs = append(testCreatedProductIDs, created.Id)

	// Now update the product
	updateBody := httpdelivery.ProductRequest{Name: "New Name", Price: 200.0, Quantity: 2}
	jsonUpdateBody, _ := json.Marshal(updateBody)
	updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/products/%d", created.Id), bytes.NewReader(jsonUpdateBody))
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", updateW.Code)
	}

	var updated httpdelivery.ProductResponse
	if err := json.NewDecoder(updateW.Body).Decode(&updated); err != nil {
		t.Fatalf("error decoding update response: %v", err)
	}

	if updated.Name != "New Name" {
		t.Errorf("expected name 'New Name', got %v", updated.Name)
	}
	if updated.Price != 200.0 {
		t.Errorf("expected price 200.0, got %v", updated.Price)
	}
	if updated.Quantity != 2 {
		t.Errorf("expected quantity 2, got %v", updated.Quantity)
	}
}

func TestUpdateProductHandler_NotFound(t *testing.T) {
	r := httpdelivery.NewRouter()
	updateBody := httpdelivery.ProductRequest{Name: "Ghost", Price: 1.0}
	jsonBody, _ := json.Marshal(updateBody)
	req := httptest.NewRequest(http.MethodPut, "/products/999999", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Code)
	}
}

func TestUpdateProductHandler_InvalidInput(t *testing.T) {
	r := httpdelivery.NewRouter()
	invalidJSON := `{Name: "Bad" Price: 999}` // missing comma
	req := httptest.NewRequest(http.MethodPut, "/products/1", bytes.NewBufferString(invalidJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}
}

func TestUpdateProductHandler_ValidationErrors(t *testing.T) {
	t.Cleanup(cleanupCreatedProducts)
	r := httpdelivery.NewRouter()

	// Create valid product
	createBody := httpdelivery.ProductRequest{Name: "Temporary", Price: 100.0, Quantity: 1}
	jsonCreateBody, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreateBody))
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", createW.Code)
	}
	var created httpdelivery.ProductResponse
	json.NewDecoder(createW.Body).Decode(&created)

	// Store the created product ID for cleanup
	testCreatedProductIDs = append(testCreatedProductIDs, created.Id)

	// Try invalid update
	invalidUpdate := httpdelivery.ProductRequest{Name: "", Price: -100, Quantity: -1}
	jsonInvalid, _ := json.Marshal(invalidUpdate)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/products/%d", created.Id), bytes.NewReader(jsonInvalid))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}

	var resp map[string]map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	errorsMap := resp["errors"]
	if _, ok := errorsMap["name"]; !ok {
		t.Errorf("expected validation error for 'name'")
	}
	if _, ok := errorsMap["price"]; !ok {
		t.Errorf("expected validation error for 'price'")
	}
	if _, ok := errorsMap["quantity"]; !ok {
		t.Errorf("expected validation error for 'quantity'")
	}
}

// cleanupCreatedProducts deletes all products created during tests.
func cleanupCreatedProducts() {
	r := httpdelivery.NewRouter()
	for _, id := range testCreatedProductIDs {
		deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/products/%d", id), nil)
		deleteW := httptest.NewRecorder()
		r.ServeHTTP(deleteW, deleteReq)
	}
	testCreatedProductIDs = nil // reset after cleanup
}
