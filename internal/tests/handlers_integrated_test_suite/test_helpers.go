package handlers_integrated_test_suite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/rogerio-castellano/inventory-tracker/internal/db"
	api "github.com/rogerio-castellano/inventory-tracker/internal/http"
	handler "github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
	"github.com/rogerio-castellano/inventory-tracker/internal/models"
	"github.com/rogerio-castellano/inventory-tracker/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

var (
	token        string
	productRepo  *repo.PostgresProductRepository
	movementRepo *repo.PostgresMovementRepository
	database     *sql.DB
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
	var err error
	database, err = db.Connect()
	if err != nil {
		log.Fatal("❌ Could not connect to database:", err)
	}

	productRepo = repo.NewPostgresProductRepository(database)
	handler.SetProductRepo(productRepo)

	movementRepo = repo.NewPostgresMovementRepository(database)
	handler.SetMovementRepo(movementRepo)

	userRepo := repo.NewPostgresUserRepository(database)
	handler.SetUserRepo(userRepo)

	createAdminIfNotExists(userRepo, password)

	metricsRepo := repo.NewPostgresMetricsRepository(database)
	handler.SetMetricsRepo(metricsRepo)
	// metricsRepo.SetRepositories(productRepo, movementRepo)
}

func createAdminIfNotExists(userRepo *repo.PostgresUserRepository, password string) {
	exists, err := userExists("admin")
	if err != nil {
		fmt.Println("error checking if admin exists", err)
	}

	if !exists {
		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		userRepo.CreateUser(models.User{
			Username:     "admin",
			PasswordHash: string(hash),
		})
	}
}

func userExists(username string) (bool, error) {
	const query = `SELECT COUNT(*) FROM users WHERE username = $1`

	var count int
	err := database.QueryRow(query, username).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query failed: %w", err)
	}
	return count > 0, nil
}

func clearAllProducts() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := database.ExecContext(ctx, "TRUNCATE TABLE products RESTART IDENTITY CASCADE")
	if err != nil {
		fmt.Println(fmt.Errorf("failed to truncate products table: %w", err))
	}
}

func clearAllUsersExceptAdmin() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := database.ExecContext(ctx, "DELETE FROM users WHERE username <> 'admin'")
	if err != nil {
		fmt.Println(fmt.Errorf("failed to delete users: %w", err))
	}
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

func addMovement(m models.Movement) {
	query := `INSERT INTO movements (product_id, delta, created_at, updated_at) VALUES ($1, $2, $3, $4)`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := database.ExecContext(ctx, query, m.ProductID, m.Delta, m.CreatedAt, m.CreatedAt)
	if err != nil {
		log.Println("Error adding a movement %w", err)
	}
}
