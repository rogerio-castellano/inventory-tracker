package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	handler "github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
	"github.com/rogerio-castellano/inventory-tracker/internal/models"
	repo "github.com/rogerio-castellano/inventory-tracker/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

var movementRepo *repo.InMemoryMovementRepository
var productRepo *repo.InMemoryProductRepository
var token string

func init() {
	testPassword := "secret"
	setupTestRepo(testPassword)
	r := api.NewRouter()
	newToken, err := generateToken(r, "admin", testPassword)
	if err != nil {
		panic(fmt.Sprintf("error generating token: %v", err))
	}

	token = newToken
}

func setupTestRepo(testPassword string) {
	productRepo = repo.NewInMemoryProductRepository()
	handler.SetProductRepo(productRepo)

	movementRepo = repo.NewInMemoryMovementRepository()
	handler.SetMovementRepo(movementRepo)

	userRepo := repo.NewInMemoryUserRepository()
	handler.SetUserRepo(userRepo)

	// Pre-populate with an admin user
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	userRepo.CreateUser(models.User{
		Username:     "admin",
		PasswordHash: string(passwordHash),
	})

	metricsRepo := repo.NewInMemoryMetricsRepository()
	handler.SetMetricsRepo(metricsRepo)
	metricsRepo.SetRepositories(productRepo, movementRepo)
}

func clearAllProducts() {
	productRepo.Clear()
}

func generateToken(r http.Handler, username, password string) (string, error) {
	payload := handler.UserLogin{Username: username, Password: password}
	body, _ := json.Marshal(payload)
	loginReq := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)

	var resp handler.LoginResult
	err := json.NewDecoder(loginW.Body).Decode(&resp)
	if err != nil {
		return "", fmt.Errorf("failed to decode token response: %v", err)
	}

	return resp.Token, nil
}

func createProduct(r http.Handler, product handler.ProductRequest) *httptest.ResponseRecorder {
	jsonBody, _ := json.Marshal(product)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}

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

func adjustProduct(r http.Handler, productID int, adjustment handler.QuantityAdjustmentRequest) *httptest.ResponseRecorder {
	jsonBody, _ := json.Marshal(adjustment)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", productID), bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}

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
	movementRepo.AddMovement(created.Id, 5, time.Now().Add(-48*time.Hour).UTC())

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
	movementRepo.AddMovement(created.Id, -1, time.Now().Add(-72*time.Hour).UTC())

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

func TestAuthFlow(t *testing.T) {
	r := api.NewRouter()

	t.Run("Login with valid credentials", func(t *testing.T) {
		payload := handler.UserLogin{Username: "admin", Password: "secret"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.LoginResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode token response: %v", err)
		}
		if resp.Token == "" {
			t.Error("expected token in response")
		}
	})

	t.Run("Protected route without token is rejected", func(t *testing.T) {
		product := handler.ProductRequest{Name: "AuthBox", Price: 999.0, Quantity: 1}
		b, _ := json.Marshal(product)
		req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(b))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("Protected route with valid token succeeds", func(t *testing.T) {
		payload := handler.UserLogin{Username: "admin", Password: "secret"}
		body, _ := json.Marshal(payload)
		loginReq := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		loginW := httptest.NewRecorder()
		r.ServeHTTP(loginW, loginReq)

		var resp handler.LoginResult
		_ = json.NewDecoder(loginW.Body).Decode(&resp)
		token := resp.Token

		product := handler.ProductRequest{Name: "SecureProduct", Price: 10.0, Quantity: 2}
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
		data := handler.UserLogin{
			Username: "testuser",
			Password: "strongpassword",
		}
		body, _ := json.Marshal(data)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d", w.Code)
		}
		var resp handler.RegisterResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Token == "" {
			t.Error("expected token in response")
		}
	})

	t.Run("Duplicate username returns 409", func(t *testing.T) {
		data := handler.UserLogin{
			Username: "testuser",
			Password: "anotherpass",
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
		data := handler.UserLogin{
			Username: "shortpass",
			Password: "123",
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

func TestImportProductsHandler(t *testing.T) {
	r := api.NewRouter()

	t.Run("File with unique valid products", func(t *testing.T) {

		t.Cleanup(clearAllProducts)
		// Create CSV data (2 valid)
		csvData := `name,price,quantity,threshold
Mouse,25.99,10,2
Keyboard,45.00,5,1`

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "products.csv")
		part.Write([]byte(csvData))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/products/import", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.ImportProductsResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		imported := resp.ImportedProductsCount

		if imported != 2 {
			t.Errorf("expected 2 imported products, got %d", imported)
		}
		if len(resp.Errors) != 0 {
			t.Errorf("expected no errors, got %v", resp.Errors)
		}
	})

	t.Run("File with one invalid product", func(t *testing.T) {
		t.Cleanup(clearAllProducts)

		// Create CSV data (2 valid, 1 invalid)
		csvData := `name,price,quantity,threshold
Mouse,25.99,10,2
InvalidProduct,0,3,1
Keyboard,45.00,5,1`

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "products.csv")
		part.Write([]byte(csvData))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/products/import", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.ImportProductsResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		imported := resp.ImportedProductsCount
		errors := resp.Errors

		if imported != 2 {
			t.Errorf("expected 2 imported products, got %d", imported)
		}
		if len(errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(errors))
		} else if !strings.Contains(resp.Errors[0].Description, "row 3") {
			t.Errorf("expected error for row 3, got %v", errors[0])
		}

		wanterrorContains := "invalid values"
		if !strings.Contains(resp.Errors[0].Description, wanterrorContains) {
			t.Errorf("expected first error to constains %s , got %s", wanterrorContains, errors[0])
		}
	})

	t.Run("File with a duplicated product (Mouse) in default mode (skip)", func(t *testing.T) {
		t.Cleanup(clearAllProducts)

		// Create CSV data (2 valid, 2 invalid)
		csvData := `name,price,quantity,threshold
	Mouse,25.99,10,2
	Keyboard,45.00,5,1
	Mouse,19.00,4,2`

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "products.csv")
		part.Write([]byte(csvData))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/products/import", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.ImportProductsResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.ImportedProductsCount != 2 {
			t.Errorf("expected 2 imported products, got %d", resp.ImportedProductsCount)
		}
		if len(resp.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(resp.Errors))
		}

		wantErrorContains := "already exists"
		if !strings.Contains(resp.Errors[0].Description, wantErrorContains) {
			t.Errorf("expected error to constains %s , got %s", wantErrorContains, resp.Errors[0].Description)
		}
	})

	t.Run("File with a duplicated product (Mouse) providing explicitly the skip mode", func(t *testing.T) {
		t.Cleanup(clearAllProducts)

		// Create CSV data (2 valid, 2 invalid)
		csvData := `name,price,quantity,threshold
	Mouse,25.99,10,2
	Keyboard,45.00,5,1
	Mouse,19.00,4,2`

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "products.csv")
		part.Write([]byte(csvData))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/products/import?mode=skip", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.ImportProductsResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.ImportedProductsCount != 2 {
			t.Errorf("expected 2 imported products, got %d", resp.ImportedProductsCount)
		}
		errors := resp.Errors
		if len(errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(errors))
		}

		wantErrorContains := "already exists"
		if !strings.Contains(errors[0].Description, wantErrorContains) {
			t.Errorf("expected error to constains %s , got %s", wantErrorContains, errors[0])
		}
	})

	t.Run("Import with update mode replaces product", func(t *testing.T) {
		// Create a product to update
		original := handler.ProductRequest{Name: "Monitor", Price: 200.0, Quantity: 5, Threshold: 2}
		createProduct(r, original)

		// Import CSV with same product name but new values
		csv := `name,price,quantity,threshold
Monitor,99.0,1,1`

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "update.csv")
		part.Write([]byte(csv))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/products/import?mode=update", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handler.ImportProductsResult
		json.NewDecoder(w.Body).Decode(&resp)

		if resp.ImportedProductsCount != 1 {
			t.Errorf("expected 1 update, got %v", resp.ImportedProductsCount)
		}

		// Check updated product
		get := httptest.NewRequest(http.MethodGet, "/products", nil)
		getW := httptest.NewRecorder()
		r.ServeHTTP(getW, get)

		var all []handler.ProductResponse
		json.NewDecoder(getW.Body).Decode(&all)

		for _, p := range all {
			if p.Name == "Monitor" {
				if p.Price != 99.0 {
					t.Errorf("expected updated price 99.0, got %v", p.Price)
				}
			}
		}
	})
}
