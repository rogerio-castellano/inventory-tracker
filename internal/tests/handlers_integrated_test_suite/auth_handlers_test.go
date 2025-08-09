package handlers_integrated_test_suite

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	httpapp "github.com/rogerio-castellano/inventory-tracker/internal/http"
	"github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
)

func runWithVisitorCleanup(t *testing.T, name string, testFunc func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		httpapp.CleanupAllVisitors()
		testFunc(t)
	})
}

func TestAuthFlow(t *testing.T) {
	r := api.NewRouter()

	runWithVisitorCleanup(t, "Login with valid credentials", func(t *testing.T) {
		payload := handlers.CredentialsRequest{Username: "admin", Password: "secret"}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp handlers.LoginResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode token response: %v", err)
		}
		if resp.AccessToken == "" {
			t.Error("expected access token in response")
		}
		if resp.RefreshToken == "" {
			t.Error("expected refresh token in response")
		}
	})

	runWithVisitorCleanup(t, "Protected route without token is rejected", func(t *testing.T) {
		t.Cleanup(clearAllProducts)

		product := handlers.ProductRequest{Name: "AuthBox", Price: 999.0, Quantity: 1}
		b, _ := json.Marshal(product)
		req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(b))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	runWithVisitorCleanup(t, "Protected route with valid token succeeds", func(t *testing.T) {
		t.Cleanup(clearAllProducts)
		payload := handlers.CredentialsRequest{Username: "admin", Password: "secret"}
		body, _ := json.Marshal(payload)
		loginReq := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		loginW := httptest.NewRecorder()
		r.ServeHTTP(loginW, loginReq)

		var resp handlers.LoginResult
		_ = json.NewDecoder(loginW.Body).Decode(&resp)
		token := resp.AccessToken

		product := handlers.ProductRequest{Name: "SecureProduct", Price: 10.0, Quantity: 2}
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

	runWithVisitorCleanup(t, "Valid registration returns token", func(t *testing.T) {
		t.Cleanup(clearAllUsersExceptAdmin)
		data := handlers.CredentialsRequest{Username: "testuser", Password: "strongpassword"}
		body, _ := json.Marshal(data)
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d", w.Code)
		}
		var resp handlers.RegisterResult
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Token == "" {
			t.Error("expected token in response")
		}
	})

	runWithVisitorCleanup(t, "Duplicate username returns 409", func(t *testing.T) {
		t.Cleanup(clearAllUsersExceptAdmin)
		data := handlers.CredentialsRequest{Username: "testuser", Password: "anotherpass"}
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

	runWithVisitorCleanup(t, "Too short password returns 400", func(t *testing.T) {
		data := handlers.CredentialsRequest{Username: "shortpass",
			Password: "123"}
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
