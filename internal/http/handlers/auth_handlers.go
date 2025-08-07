package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/rogerio-castellano/inventory-tracker/internal/auth"
	"github.com/rogerio-castellano/inventory-tracker/internal/models"
	"github.com/rogerio-castellano/inventory-tracker/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

// var userRepo repo.UserRepository

// func SetUserRepo(r repo.UserRepository) {
// 	userRepo = r
// }

// RegisterHandler godoc
// @Summary Register new user and return JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body map[string]string true "username and password"
// @Success 201 {object} map[string]string
// @Failure 400 {string} string "Invalid input"
// @Failure 409 {string} string "User exists"
// @Router /register [post]
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var creds CredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	if creds.Username == "" || creds.Password == "" {
		http.Error(w, "Missing credentials", http.StatusBadRequest)
		return
	}

	if len(creds.Username) < 3 || len(creds.Password) < 6 {
		http.Error(w, "username or password too short", http.StatusBadRequest)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	user := models.User{
		Username:     creds.Username,
		PasswordHash: string(hashed),
		Role:         "user",
	}

	_, err = userRepo.CreateUser(user)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			http.Error(w, "username already exists", http.StatusConflict)
		} else {
			http.Error(w, "failed to register user", http.StatusInternalServerError)
		}
		return
	}

	// Generate a token for the new user
	token, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(RegisterResult{
		Message: "user registered",
		Token:   token,
	})

	if err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// @Summary Create user with custom role
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param user body RegisterAsAdminRequest true "User to create with role"
// @Success 201 {object} map[string]string
// @Failure 400 {string} string "Invalid input"
// @Failure 403 {string} string "Forbidden"
// @Failure 409 {string} string "User exists"
// @Failure 500 {string} string "Server error"
// @Router /admin/users [post]
func RegisterAsAdminHandler(w http.ResponseWriter, r *http.Request) {
	role, err := GetRoleFromContext(r)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req RegisterAsAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" || req.Role == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}

	user := models.User{
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
		Role:         req.Role,
	}

	if _, err := userRepo.CreateUser(user); err != nil {
		if errors.Is(err, repo.ErrDuplicatedValueUnique) {
			http.Error(w, "could not create user: username duplicated", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Error creating user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(map[string]string{
		"message": "User created",
	})

	if err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// LoginHandler godoc
// @Summary Authenticate user and return JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body map[string]string true "username and password"
// @Success 200 {object} map[string]string
// @Failure 400 {string} string "Invalid input"
// @Failure 401 {string} string "Unauthorized"
// @Router /login [post]
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var credentials CredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	user, err := userRepo.GetByUsername(credentials.Username)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(credentials.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "could not generate token", http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(LoginResult{Token: token})

	if err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}
