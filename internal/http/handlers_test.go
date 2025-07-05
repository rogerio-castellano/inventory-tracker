package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	"github.com/rogerio-castellano/inventory-tracker/internal/models"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
)

var movementRepo *repo.InMemoryMovementRepository
var token string

func init() {
	setupTestRepo()
	r := api.NewRouter()
	newToken, err := generateToken(r, "admin", "secret")
	if err != nil {
		panic(fmt.Sprintf("error generating token: %v", err))
	}

	token = newToken
}

func setupTestRepo() {
	api.SetProductRepo(repo.NewInMemoryProductRepository())
	movementRepo = repo.NewInMemoryMovementRepository()
	api.SetMovementRepo(movementRepo)
	userRepo := repo.NewInMemoryUserRepository()
	api.SetUserRepo(userRepo)
	// Pre-populate with an admin user
	userRepo.CreateUser(models.User{
		Username:     "admin",
		PasswordHash: "$2y$10$Q20mohJyT0SatlDqMxQLN.cQF6oN6EGjq2fl8Hemm4mC2sfQv7gF.", // bcrypt hash of "secret"
	})
}

func TestCreateProductHandler_Valid(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()
	body := api.ProductRequest{Name: "Laptop", Price: 1500.0, Quantity: 1}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}

	var resp api.ProductResponse
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
		payload        api.ProductRequest
		expectCode     int
		expectedErrors []string
	}{
		{
			name:           "Empty name and price",
			payload:        api.ProductRequest{Name: "", Price: 0.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"name", "price"},
		},
		{
			name:           "Empty name only",
			payload:        api.ProductRequest{Name: "", Price: 100.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"name"},
		},
		{
			name:           "Invalid price only",
			payload:        api.ProductRequest{Name: "Mouse", Price: -5.0},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"price"},
		},
		{
			name:           "Negative quantity",
			payload:        api.ProductRequest{Name: "Keyboard", Price: 50.0, Quantity: -1},
			expectCode:     http.StatusBadRequest,
			expectedErrors: []string{"quantity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBody, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonBody))
			req.Header.Set("Authorization", "Bearer "+token)
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

	// Create products to ensure we have something to retrieve
	createBody := api.ProductRequest{Name: "Phone", Price: 999.99, Quantity: 1}
	jsonCreateBody, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreateBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for product creation, got %d", createW.Code)
	}

	// Create a second product
	createBody2 := api.ProductRequest{Name: "Tablet", Price: 499.99, Quantity: 2}
	jsonCreateBody2, _ := json.Marshal(createBody2)
	createReq2 := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreateBody2))
	createReq2.Header.Set("Authorization", "Bearer "+token)
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

	var products []api.ProductResponse
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

	// First, create a product
	createBody := api.ProductRequest{Name: "Old Name", Price: 100.0, Quantity: 1}
	jsonCreateBody, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreateBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", createW.Code)
	}

	var created api.ProductResponse
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("error decoding create response: %v", err)
	}

	// Now update the product
	updateBody := api.ProductRequest{Name: "New Name", Price: 200.0, Quantity: 2}
	jsonUpdateBody, _ := json.Marshal(updateBody)
	updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/products/%d", created.Id), bytes.NewReader(jsonUpdateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", updateW.Code)
	}

	var updated api.ProductResponse
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
	updateBody := api.ProductRequest{Name: "Ghost", Price: 1.0}
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
	// Create valid product
	createBody := api.ProductRequest{Name: "Temporary", Price: 100.0, Quantity: 1}
	jsonCreateBody, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreateBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", createW.Code)
	}
	var created api.ProductResponse
	json.NewDecoder(createW.Body).Decode(&created)

	// Try invalid update
	invalidUpdate := api.ProductRequest{Name: "", Price: -100, Quantity: -1}
	jsonInvalid, _ := json.Marshal(invalidUpdate)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/products/%d", created.Id), bytes.NewReader(jsonInvalid))
	req.Header.Set("Authorization", "Bearer "+token)
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

func TestFilterProductsHandler(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()
	// Seed test data
	products := []api.ProductRequest{
		{Name: "Phone", Price: 699.99, Quantity: 10},
		{Name: "Laptop", Price: 1299.99, Quantity: 5},
		{Name: "Mouse", Price: 29.99, Quantity: 50},
		{Name: "Monitor", Price: 199.99, Quantity: 20},
	}

	for _, p := range products {
		body, _ := json.Marshal(p)
		req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
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
		var resp api.ProductsSearchResult
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
		var resp api.ProductsSearchResult
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
		var resp api.ProductsSearchResult
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
		var resp api.ProductsSearchResult
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
		var resp api.ProductsSearchResult
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
		var resp api.ProductsSearchResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("error decoding response: %v", err)
		}
		if got := len(resp.Data); got != 0 {
			t.Errorf("expected empty result, got %d items", got)
		}
	})
}

func TestAdjustQuantityHandler(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()
	// Create a product
	create := api.ProductRequest{Name: "InventoryItem", Price: 10.0, Quantity: 10}
	body, _ := json.Marshal(create)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created api.ProductRequest
	json.NewDecoder(w.Body).Decode(&created)

	t.Run("Increase quantity", func(t *testing.T) {
		adj := api.QuantityAdjustmentRequest{Delta: 5}
		body, _ := json.Marshal(adj)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", created.Id), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var resp api.ProductResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Quantity != 15 {
			t.Errorf("expected quantity 15, got %v", resp.Quantity)
		}
	})

	t.Run("Decrease quantity", func(t *testing.T) {
		adj := api.QuantityAdjustmentRequest{Delta: -3}
		body, _ := json.Marshal(adj)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", created.Id), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var resp api.ProductResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if resp.Quantity != 12.0 {
			t.Errorf("expected quantity 12, got %v", resp.Quantity)
		}
	})

	t.Run("Too much decrease (underflow)", func(t *testing.T) {
		adj := api.QuantityAdjustmentRequest{Delta: -100}
		body, _ := json.Marshal(adj)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", created.Id), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict, got %d", w.Code)
		}
	})

	t.Run("Invalid ID", func(t *testing.T) {
		adj := api.QuantityAdjustmentRequest{Delta: 1}
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

func TestGetMovementsHandler(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	// Create a product
	product := api.ProductRequest{Name: "Box", Price: 50.0, Quantity: 10}
	body, _ := json.Marshal(product)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created api.ProductResponse
	json.NewDecoder(createW.Body).Decode(&created)

	// Adjust quantity twice to generate movement log
	adjust := func(delta int) {
		adj := api.QuantityAdjustmentRequest{Delta: delta}
		body, _ := json.Marshal(adj)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", created.Id), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
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

		var movementsCollection api.MovementsSearchResult
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

		var movements api.MovementsSearchResult
		json.NewDecoder(w.Body).Decode(&movements)
		if count := movements.Meta.TotalCount; count != 0 {
			t.Errorf("expected 0 movements, got %d", count)
		}
	})
}

func TestGetMovementsHandler_Filtering(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	// Create a product
	create := api.ProductRequest{Name: "FilterBox", Price: 80.0, Quantity: 10}
	jsonCreate, _ := json.Marshal(create)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreate))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created api.ProductResponse
	json.NewDecoder(createW.Body).Decode(&created)

	// First adjustment: backdated (manually insert)
	movementRepo.AddMovement(created.Id, 5, time.Now().Add(-48*time.Hour).UTC())

	// Second adjustment: recent
	adj := api.MovementResponse{Delta: 2}
	jsonAdj, _ := json.Marshal(adj)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", created.Id), bytes.NewReader(jsonAdj))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("failed to adjust product")
	}

	t.Run("since: only recent movement", func(t *testing.T) {
		since := time.Now().Add(-12 * time.Hour).Format(time.RFC3339)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d/movements?since=%s", created.Id, since), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var movementsCollection api.MovementsSearchResult
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

		var movementsCollection api.MovementsSearchResult
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

		var movementsCollection api.MovementsSearchResult
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

		var movementsCollection api.MovementsSearchResult
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

	// Create a product
	create := api.ProductRequest{Name: "PagedWidget", Price: 20.0, Quantity: 5}
	jsonCreate, _ := json.Marshal(create)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreate))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created api.ProductResponse
	json.NewDecoder(createW.Body).Decode(&created)

	// Generate 3 movements
	deltas := []int{+1, -1, +2}
	for _, d := range deltas {
		adj := api.QuantityAdjustmentRequest{Delta: d}
		b, _ := json.Marshal(adj)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", created.Id), bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
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

		var resp api.MovementsSearchResult
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

		var resp api.MovementsSearchResult
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

		var resp api.MovementsSearchResult
		json.NewDecoder(w.Body).Decode(&resp)
		items := resp.Data
		if len(items) != 1 {
			t.Errorf("expected 1 item, got %d", len(items))
		}
	})
}

func TestExportMovementsHandler(t *testing.T) {
	t.Cleanup(clearAllProducts)
	r := api.NewRouter()

	// Create a product
	product := api.ProductRequest{Name: "Exportable", Price: 100.0, Quantity: 5}
	b, _ := json.Marshal(product)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(b))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created api.ProductResponse
	json.NewDecoder(createW.Body).Decode(&created)

	// Add 1 movement
	adj := api.QuantityAdjustmentRequest{Delta: 3}
	body, _ := json.Marshal(adj)
	adjReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", created.Id), bytes.NewReader(body))
	adjReq.Header.Set("Authorization", "Bearer "+token)
	adjW := httptest.NewRecorder()
	r.ServeHTTP(adjW, adjReq)
	if adjW.Code != http.StatusOK {
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

	// Create a product
	create := api.ProductRequest{Name: "FilteredExport", Price: 75.0, Quantity: 8}
	jsonCreate, _ := json.Marshal(create)
	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonCreate))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create product")
	}
	var created api.ProductResponse
	json.NewDecoder(createW.Body).Decode(&created)

	// Insert one old movement
	movementRepo.AddMovement(created.Id, -1, time.Now().Add(-72*time.Hour).UTC())

	// Insert one recent movement via API
	adj := api.QuantityAdjustmentRequest{Delta: 2}
	body, _ := json.Marshal(adj)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", created.Id), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
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
		var items []map[string]any
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

func TestAuthFlow(t *testing.T) {
	r := api.NewRouter()

	t.Run("Login with valid credentials", func(t *testing.T) {
		payload := map[string]string{"username": "admin", "password": "secret"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp map[string]string
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode token response: %v", err)
		}
		if resp["token"] == "" {
			t.Error("expected token in response")
		}
	})

	t.Run("Protected route without token is rejected", func(t *testing.T) {
		product := map[string]any{"name": "AuthBox", "price": 999.0, "quantity": 1}
		b, _ := json.Marshal(product)
		req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(b))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("Protected route with valid token succeeds", func(t *testing.T) {
		// login to get token
		payload := map[string]string{"username": "admin", "password": "secret"}
		body, _ := json.Marshal(payload)
		loginReq := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		loginW := httptest.NewRecorder()
		r.ServeHTTP(loginW, loginReq)

		var resp map[string]string
		_ = json.NewDecoder(loginW.Body).Decode(&resp)
		token := resp["token"]

		// make authorized request
		product := map[string]any{"name": "SecureProduct", "price": 10.0, "quantity": 2}
		b, _ := json.Marshal(product)
		req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d", w.Code)
		}
	})
}

func TestRegisterHandler(t *testing.T) {
	r := api.NewRouter()

	t.Run("Valid registration returns token", func(t *testing.T) {
		data := map[string]string{
			"username": "testuser",
			"password": "strongpassword",
		}
		body, _ := json.Marshal(data)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d", w.Code)
		}
		var resp map[string]string
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["token"] == "" {
			t.Error("expected token in response")
		}
	})

	t.Run("Duplicate username returns 409", func(t *testing.T) {
		data := map[string]string{
			"username": "testuser", // same as above
			"password": "anotherpass",
		}
		body, _ := json.Marshal(data)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict, got %d", w.Code)
		}
	})

	t.Run("Too short password returns 400", func(t *testing.T) {
		data := map[string]string{
			"username": "shortpass",
			"password": "123",
		}
		body, _ := json.Marshal(data)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("Malformed JSON returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

// clearAllProducts removes all products using the HTTP API endpoints.
func clearAllProducts() {
	r := api.NewRouter()
	getReq := httptest.NewRequest(http.MethodGet, "/products", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		return // nothing to clear or error
	}
	var products []api.ProductResponse
	if err := json.NewDecoder(getW.Body).Decode(&products); err != nil {
		return
	}
	for _, p := range products {
		id := fmt.Sprintf("%v", p.Id)
		deleteReq := httptest.NewRequest(http.MethodDelete, "/products/"+id, nil)
		deleteReq.Header.Set("Authorization", "Bearer "+token)
		deleteW := httptest.NewRecorder()
		r.ServeHTTP(deleteW, deleteReq)
	}
}

func generateToken(r http.Handler, username, password string) (string, error) {
	payload := map[string]string{"username": "admin", "password": "secret"}
	body, _ := json.Marshal(payload)
	loginReq := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)

	var resp map[string]string
	err := json.NewDecoder(loginW.Body).Decode(&resp)
	if err != nil {
		return "", fmt.Errorf("failed to decode token response: %v", err)
	}

	return resp["token"], nil
}
