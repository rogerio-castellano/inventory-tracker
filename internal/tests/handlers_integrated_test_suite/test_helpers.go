package handlers_integrated_test_suite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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
	userRepo     *repo.PostgresUserRepository
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
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		os.Setenv("DATABASE_URL", "postgres://postgres:example@localhost:5432/inventory?sslmode=disable")
	}
	var err error
	database, err = db.Connect()
	if err != nil {
		log.Fatal("❌ Could not connect to database:", err)
	}

	productRepo = repo.NewPostgresProductRepository(database)
	handler.SetProductRepo(productRepo)

	movementRepo = repo.NewPostgresMovementRepository(database)
	handler.SetMovementRepo(movementRepo)

	userRepo = repo.NewPostgresUserRepository(database)
	handler.SetUserRepo(userRepo)

	createAdminIfNotExists(password)

	metricsRepo := repo.NewPostgresMetricsRepository(database)
	handler.SetMetricsRepo(metricsRepo)
}

func createAdminIfNotExists(password string) {
	exists, err := userExists("admin")
	if err != nil {
		fmt.Println("error checking if admin exists", err)
	}

	if !exists {
		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		userRepo.CreateUser(models.User{
			Username:     "admin",
			PasswordHash: string(hash),
			Role:         "admin",
		})
	}
}

func userRoleToken(r http.Handler) (string, error) {
	password := "secret-password"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := models.User{
		Username:     "UserRole",
		PasswordHash: string(hash),
	}
	userRepo.CreateUser(user)

	token, err := generateToken(r, user.Username, password)
	if err != nil {
		return "", err
	}

	return token, nil
}

func generateToken(r http.Handler, username, password string) (string, error) {
	payload := handler.CredentialsRequest{Username: username, Password: password}
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

func addMovement(m models.Movement) {
	query := `INSERT INTO movements (product_id, delta, created_at, updated_at) VALUES ($1, $2, $3, $4)`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := database.ExecContext(ctx, query, m.ProductID, m.Delta, m.CreatedAt, m.CreatedAt)
	if err != nil {
		log.Println("Error adding a movement %w", err)
	}
}
