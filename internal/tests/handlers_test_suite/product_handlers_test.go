package handlers_test_suite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	handler "github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
)

func TestCreateProductHandler_Valid(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	w := createProduct(r, handler.ProductRequest{Name: "Laptop", Price: 1500.0, Quantity: 1})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}

	var resp handler.ProductResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}

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
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	tests := []struct {
		name           string
		payload        handler.ProductRequest
		expectCode     int
		expectedErrors []string
	}{
		{
			name:           "Empty name and price",
			payload:        handler.ProductRequest{Name: "", Price: 0.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"Name", "Price"},
		},
		{
			name:           "Empty name only",
			payload:        handler.ProductRequest{Name: "", Price: 100.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"Name"},
		},
		{
			name:           "Invalid price only",
			payload:        handler.ProductRequest{Name: "Mouse", Price: -5.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"Price"},
		},
		{
			name:           "Negative quantity",
			payload:        handler.ProductRequest{Name: "Keyboard", Price: 50.0, Quantity: -1},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"Quantity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := createProduct(r, tt.payload)

			if w.Code != tt.expectCode {
				t.Errorf("expected status %d, got %d", tt.expectCode, w.Code)
			}

			var resp []handler.ProductValidationError
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("error decoding response: %v", err)
			}

			for _, field := range tt.expectedErrors {
				found := false
				for _, err := range resp {
					if strings.EqualFold(err.Field, field) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error for field %q, but not found", field)
				}
			}
		})
	}
}

func TestCreateProductHandler_MalformedJSON(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	badJSON := `{Name: "Invalid" Price: 100 "}` // missing comma
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(badJSON))
	req.Header.Set("Authorization", "Bearer "+token)
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
	r := api.NewRouter()

	// Create the first product
	product1 := handler.ProductRequest{Name: "Phone", Price: 999.99, Quantity: 1}
	w1 := createProduct(r, product1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for product creation, got %d", w1.Code)
	}

	// Create a second product
	product2 := handler.ProductRequest{Name: "Tablet", Price: 499.99, Quantity: 2}
	w2 := createProduct(r, product2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for second product creation, got %d", w2.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/products", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for product retrieval, got %d", getW.Code)
	}

	var products []handler.ProductResponse
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
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()
	product := handler.ProductRequest{Name: "Old Name", Price: 100.0, Quantity: 1}
	w := createProduct(r, product)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}

	var created handler.ProductResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("error decoding create response: %v", err)
	}

	updateBody := handler.ProductRequest{Name: "New Name", Price: 200.0, Quantity: 2}
	jsonUpdateBody, _ := json.Marshal(updateBody)
	updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/products/%d", created.Id), bytes.NewReader(jsonUpdateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", updateW.Code)
	}

	var updated handler.ProductResponse
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
	r := api.NewRouter()
	updateBody := handler.ProductRequest{Name: "Ghost", Price: 1.0}
	jsonBody, _ := json.Marshal(updateBody)
	req := httptest.NewRequest(http.MethodPut, "/products/999999", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Code)
	}
}

func TestUpdateProductHandler_InvalidInput(t *testing.T) {
	r := api.NewRouter()
	invalidJSON := `{Name: "Bad" Price: 999}` // missing comma
	req := httptest.NewRequest(http.MethodPut, "/products/1", bytes.NewBufferString(invalidJSON))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}
}

func TestUpdateProductHandler_ValidationErrors(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	product := handler.ProductRequest{Name: "Temporary", Price: 100.0, Quantity: 1}
	w := createProduct(r, product)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}
	var created handler.ProductResponse
	json.NewDecoder(w.Body).Decode(&created)

	// Try invalid update
	invalidUpdate := handler.ProductRequest{Name: "", Price: -100, Quantity: -1}
	jsonInvalid, _ := json.Marshal(invalidUpdate)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/products/%d", created.Id), bytes.NewReader(jsonInvalid))
	req.Header.Set("Authorization", "Bearer "+token)
	wResult := httptest.NewRecorder()
	r.ServeHTTP(wResult, req)

	if wResult.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", wResult.Code)
	}

	var resp []handler.ProductValidationError
	if err := json.NewDecoder(wResult.Body).Decode(&resp); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}

	assertField := func(field string) {
		found := false
		for _, err := range resp {
			if err.Field == field {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("expected validation error for '%v'", "Name")
		}
	}

	assertField("Name")
	assertField("Price")
	assertField("Quantity")
}

func TestFilterProductsHandler(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	products := []handler.ProductRequest{
		{Name: "Phone", Price: 699.99, Quantity: 10},
		{Name: "Laptop", Price: 1299.99, Quantity: 5},
		{Name: "Mouse", Price: 29.99, Quantity: 50},
		{Name: "Monitor", Price: 199.99, Quantity: 20},
	}

	for _, p := range products {
		w := createProduct(r, p)
		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create test product: %v", p.Name)
		}
	}

	t.Run("Filter by name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products/filter?name=phone", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp handler.ProductsSearchResult
		json.NewDecoder(w.Body).Decode(&resp)
		if len(resp.Data) != 1 || !strings.Contains(strings.ToLower(resp.Data[0].Name), "phone") {
			t.Errorf("expected one product containing 'phone', got %v", resp.Data)
		}
	})

	t.Run("Filter by price range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products/filter?minPrice=100&maxPrice=1000", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp handler.ProductsSearchResult
		json.NewDecoder(w.Body).Decode(&resp)
		for _, p := range resp.Data {
			price := p.Price
			if price < 100 || price > 1000 {
				t.Errorf("product price out of range: %v", price)
			}
		}
	})

	t.Run("Filter by quantity", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products/filter?minQty=5&maxQty=20", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp handler.ProductsSearchResult
		json.NewDecoder(w.Body).Decode(&resp)
		for _, p := range resp.Data {
			qty := p.Quantity
			if qty < 5 || qty > 20 {
				t.Errorf("quantity out of range: %v", qty)
			}
		}
	})

	t.Run("Filter with no match", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products/filter?name=xyz", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp handler.ProductsSearchResult
		json.NewDecoder(w.Body).Decode(&resp)
		if got := len(resp.Data); got != 0 {
			t.Errorf("expected empty result, got %d items", got)
		}
	})

	t.Run("Pagination limit and offset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products/filter?&offset=0&limit=2", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var resp handler.ProductsSearchResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("error decoding response: %v", err)
		}
		if got := len(resp.Data); got != 2 {
			t.Errorf("expected 2 products, got %d", got)
		}
	})

	t.Run("Pagination with no products", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products/filter?&offset=999&limit=10", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var resp handler.ProductsSearchResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("error decoding response: %v", err)
		}
		if got := len(resp.Data); got != 0 {
			t.Errorf("expected empty result, got %d items", got)
		}
	})
}
