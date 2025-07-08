package handlers_test_suite

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	handler "github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
)

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
		t.Cleanup(clearAllProducts)
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
