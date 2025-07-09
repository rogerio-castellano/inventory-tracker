package handlers_integrated_test_suite

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	handler "github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
)

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
		t.Cleanup(clearAllProducts)

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
		t.Cleanup(clearAllProducts)
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
		t.Cleanup(clearAllUsersExceptAdmin)
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
		t.Cleanup(clearAllUsersExceptAdmin)
		data := handler.UserLogin{
			Username: "testuser",
			Password: "anotherpass",
		}
		body, _ := json.Marshal(data)

		var w *httptest.ResponseRecorder
		// Register the a user
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Register the same user again
		req = httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
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
