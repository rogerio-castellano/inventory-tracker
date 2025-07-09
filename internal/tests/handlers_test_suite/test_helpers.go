package handlers_test_suite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"

	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	handler "github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
	"github.com/rogerio-castellano/inventory-tracker/internal/models"
	"github.com/rogerio-castellano/inventory-tracker/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

var (
	token        string
	productRepo  *repo.InMemoryProductRepository
	movementRepo *repo.InMemoryMovementRepository
)

func init() {
	setupTestRepos("secret")
	r := api.NewRouter()

	var err error
	token, err = generateToken(r, "admin", "secret")
	if err != nil {
		panic(fmt.Sprintf("error generating token: %v", err))
	}
}

func setupTestRepos(password string) {
	productRepo = repo.NewInMemoryProductRepository()
	handler.SetProductRepo(productRepo)

	movementRepo = repo.NewInMemoryMovementRepository()
	handler.SetMovementRepo(movementRepo)

	userRepo := repo.NewInMemoryUserRepository()
	handler.SetUserRepo(userRepo)

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	userRepo.CreateUser(models.User{
		Username:     "admin",
		PasswordHash: string(hash),
	})

	metricsRepo := repo.NewInMemoryMetricsRepository()
	handler.SetMetricsRepo(metricsRepo)
	metricsRepo.SetRepositories(productRepo, movementRepo)
}

func clearAllProducts() {
	productRepo.Clear()
}

func clearAllUsersExceptAdmin() {
	//Not necessary with non-persistent storage
}

func generateToken(r http.Handler, username, password string) (string, error) {
	payload := handler.UserLogin{Username: username, Password: password}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp handler.LoginResult
	err := json.NewDecoder(w.Body).Decode(&resp)
	if err != nil {
		return "", fmt.Errorf("token decoding failed: %v", err)
	}
	return resp.Token, nil
}

func createProduct(r http.Handler, p handler.ProductRequest) *httptest.ResponseRecorder {
	body, _ := json.Marshal(p)
	req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func adjustProduct(r http.Handler, productID int, adj handler.QuantityAdjustmentRequest) *httptest.ResponseRecorder {
	body, _ := json.Marshal(adj)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%d/adjust", productID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func multipartCSV(csvContent string, filename string) (*bytes.Buffer, string) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, _ := writer.CreateFormFile("file", filename)
	part.Write([]byte(csvContent))

	writer.Close()
	return &buf, writer.FormDataContentType()
}

func addMovement(movement models.Movement) {
	movementRepo.AddMovement(movement)
}
