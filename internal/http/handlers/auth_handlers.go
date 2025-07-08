package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rogerio-castellano/inventory-tracker/internal/auth"
	models "github.com/rogerio-castellano/inventory-tracker/internal/models"
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
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
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
	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(RegisterResult{
		Message: "user registered",
		Token:   token,
	})
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
	userLogin := UserLogin{}
	if err := json.NewDecoder(r.Body).Decode(&userLogin); err != nil {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}

	user, err := userRepo.GetByUsername(userLogin.Username)

	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(userLogin.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		http.Error(w, "could not generate token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(LoginResult{Token: token})
}
