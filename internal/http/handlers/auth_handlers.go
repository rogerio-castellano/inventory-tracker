package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rogerio-castellano/inventory-tracker/internal/auth"
	"github.com/rogerio-castellano/inventory-tracker/internal/models"
	"github.com/rogerio-castellano/inventory-tracker/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

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
	if err := readJSON(w, r, &creds); err != nil {
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

	token, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	err = writeJSON(w, http.StatusCreated, RegisterResult{
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
	if err := readJSON(w, r, &req); err != nil {
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

	err = writeJSON(w, http.StatusCreated, map[string]string{
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
	if err := readJSON(w, r, &credentials); err != nil {
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

	accessToken, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "could not generate token", http.StatusInternalServerError)
		return
	}

	refreshToken := generateRandomToken()
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "Invalid remote address", http.StatusInternalServerError)
		return
	}

	ua := r.UserAgent()
	key := sessionKey(host, ua)
	err = auth.SetRefreshToken(user.Username, key, auth.RefreshTokenEntry{
		Token:     refreshToken,
		IPAddress: host,
		UserAgent: ua,
	})
	if err != nil {
		log.Printf("Failed to set refresh token: %v", err)
	}

	err = writeJSON(w, http.StatusOK, LoginResult{AccessToken: accessToken, RefreshToken: refreshToken})
	if err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// @Summary Get current user info
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} MeResponse
// @Failure 401 {string} string "Unauthorized"
// @Router /me [get]
func MeHandler(w http.ResponseWriter, r *http.Request) {
	authorization := r.Header.Get("Authorization")
	_, claims, err := auth.TokenClaims(authorization)
	if err != nil {
		log.Printf("Error getting claims: %v", err)
	}

	resp := MeResponse{
		Username: claims["username"].(string),
		Role:     claims["role"].(string),
	}

	if err := writeJSON(w, http.StatusOK, resp); err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// @Summary Refresh access token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token data"
// @Success 200 {object} map[string]string
// @Failure 400 {string} string "Bad request"
// @Failure 401 {string} string "Invalid token"
// @Router /refresh [post]
func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "Invalid remote address", http.StatusInternalServerError)
		return
	}

	ua := r.UserAgent()
	key := sessionKey(host, ua)
	userSessions, ok, err := auth.GetRefreshToken(req.Username)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "No active sessions", http.StatusNotFound)
		return
	}
	stored, ok := userSessions[key]
	if !ok || stored.Token != req.RefreshToken {
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	if time.Since(stored.CreatedAt) > auth.RefreshTokenMaxAge {
		if err := auth.RemoveRefreshToken(req.Username, key); err != nil {
			http.Error(w, "Failed to handle refresh token", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Refresh token expired", http.StatusUnauthorized)
		return
	}

	user, err := userRepo.GetByUsername(req.Username)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	newToken, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Rotate refresh token
	newRefreshToken := generateRandomToken()
	err = auth.SetRefreshToken(user.Username, key, auth.RefreshTokenEntry{
		Token:     newRefreshToken,
		IPAddress: host,
		UserAgent: ua,
	})
	if err != nil {
		log.Printf("Failed to set refresh token: %v", err)
	}

	if err := writeJSON(w, http.StatusOK, LoginResult{AccessToken: newToken, RefreshToken: newRefreshToken}); err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// @Summary List active refresh tokens
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {array} RefreshTokenInfo
// @Failure 403 {string} string "Forbidden"
// @Router /admin/tokens [get]
func ListRefreshTokensHandler(w http.ResponseWriter, r *http.Request) {
	tokens := []RefreshTokenInfo{}
	refreshTokens, err := auth.GetRefreshTokens()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	for username, sessions := range refreshTokens {
		for _, entry := range sessions {
			tokens = append(tokens, RefreshTokenInfo{
				Username:  username,
				IssuedAt:  entry.CreatedAt,
				ExpiresAt: entry.CreatedAt.Add(auth.RefreshTokenMaxAge),
				IPAddress: entry.IPAddress,
				UserAgent: entry.UserAgent,
			})
		}
	}

	if err := writeJSON(w, http.StatusOK, tokens); err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// @Summary Revoke a user’s refresh token
// @Tags admin
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 204 "Token revoked"
// @Failure 404 {string} string "Not found"
// @Failure 403 {string} string "Forbidden"
// @Router /admin/tokens/{username} [delete]
func RevokeRefreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	_, ok, err := auth.GetRefreshToken(username)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "No active sessions", http.StatusNotFound)
		return
	}

	if err := auth.RemoveUserRefreshTokens(username); err != nil {
		http.Error(w, "Failed to handle refresh token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Logout (invalidate refresh token for this session)
// @Tags auth
// @Security BearerAuth
// @Success 204 "Logged out"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Session not found"
// @Failure 500 {string} string "Internal error"
// @Router /logout [post]
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	authorization := r.Header.Get("Authorization")
	_, claims, err := auth.TokenClaims(authorization)
	if err != nil {
		log.Printf("Error getting claims: %v", err)
	}
	username := claims["username"].(string)

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "Invalid remote address", http.StatusInternalServerError)
		return
	}

	ua := r.UserAgent()
	key := sessionKey(host, ua)
	if err := auth.RemoveRefreshToken(username, key); err != nil {
		http.Error(w, "Failed to handle refresh token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Logout from all devices (invalidate all refresh tokens)
// @Tags auth
// @Security BearerAuth
// @Success 204 "All sessions revoked"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "No active sessions"
// @Failure 500 {string} string "Internal error"
// @Router /logout/all [post]
func LogoutAllHandler(w http.ResponseWriter, r *http.Request) {
	authorization := r.Header.Get("Authorization")
	_, claims, err := auth.TokenClaims(authorization)
	if err != nil {
		log.Printf("Error getting claims: %v", err)
	}
	username := claims["username"].(string)

	_, ok, err := auth.GetRefreshToken(username)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "No active sessions", http.StatusNotFound)
		return
	}
	if err := auth.RemoveUserRefreshTokens(username); err != nil {
		http.Error(w, "Failed to handle refresh token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary List refresh tokens for a specific user
// @Tags admin
// @Security BearerAuth
// @Param username path string true "Username"
// @Produce json
// @Success 200 {array} RefreshTokenInfo
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "No sessions found"
// @Router /admin/users/{username}/tokens [get]
func ListUserTokensHandler(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	userSessions, ok, err := auth.GetRefreshToken(username)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return

	}
	if !ok {
		http.Error(w, "No active sessions", http.StatusNotFound)
		return
	}

	tokens := []RefreshTokenInfo{}
	for sessionKey, entry := range userSessions {
		tokens = append(tokens, RefreshTokenInfo{
			SessionKey: sessionKey,
			Username:   username,
			IssuedAt:   entry.CreatedAt,
			ExpiresAt:  entry.CreatedAt.Add(auth.RefreshTokenMaxAge),
			IPAddress:  entry.IPAddress,
			UserAgent:  entry.UserAgent,
		})
	}

	if err := writeJSON(w, http.StatusOK, tokens); err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// @Summary Revoke all sessions for a user
// @Tags admin
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 204 "All sessions revoked"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "No active sessions"
// @Failure 500 {string} string "Internal error"
// @Router /admin/users/{username}/tokens [delete]
func RevokeAllUserSessionsHandler(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	_, ok, err := auth.GetRefreshToken(username)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "No active sessions", http.StatusNotFound)
		return
	}

	if err := auth.RemoveUserRefreshTokens(username); err != nil {
		http.Error(w, "Failed to handle refresh token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Revoke a specific session for a user
// @Tags admin
// @Security BearerAuth
// @Param username path string true "Username"
// @Param sessionKey path string true "Session key (IP+UA hash)"
// @Success 204 "Session revoked"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "User or session not found"
// @Failure 500 {string} string "Internal error"
// @Router /admin/users/{username}/tokens/{sessionKey} [delete]
func RevokeUserSessionHandler(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	sessionKey := chi.URLParam(r, "sessionKey")

	_, ok, err := auth.GetRefreshToken(username)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return

	}
	if !ok {
		http.Error(w, "No active sessions", http.StatusNotFound)
		return
	}

	if err := auth.RemoveRefreshToken(username, sessionKey); err != nil {
		http.Error(w, "Failed to handle refresh token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Issue token for user (impersonation)
// @Tags admin
// @Security BearerAuth
// @Param username path string true "Username to impersonate"
// @Success 200 {object} map[string]string
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Failed to generate token"
// @Router /admin/users/{username}/tokens [post]
func AdminImpersonateUserHandler(w http.ResponseWriter, r *http.Request) {

	username := chi.URLParam(r, "username")

	user, err := userRepo.GetByUsername(username)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "Invalid remote address", http.StatusInternalServerError)
		return
	}
	ua := r.UserAgent()
	key := sessionKey(host, ua)

	authorization := r.Header.Get("Authorization")
	_, claims, err := auth.TokenClaims(authorization)
	if err != nil {
		log.Printf("Error getting claims: %v", err)
	}
	impersonator := claims["username"].(string)

	// Issue new access token
	accessToken, err := auth.GenerateImpersonationToken(user, impersonator)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Optional: mark token as impersonated via claims (for audit log, future)
	refreshToken := generateRandomToken()

	err = auth.SetRefreshToken(user.Username, key, auth.RefreshTokenEntry{
		Token:     refreshToken,
		IPAddress: host,
		UserAgent: ua,
	})
	if err != nil {
		log.Printf("Failed to set refresh token: %v", err)
	}

	if err := writeJSON(w, http.StatusOK, LoginResult{AccessToken: accessToken, RefreshToken: refreshToken}); err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// @Summary List all currently banned users or IPs
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {array} object
// @Failure 500 {string} string "Redis error"
// @Router /admin/bans [get]
func ListActiveBansHandler(w http.ResponseWriter, r *http.Request) {
	rdb := AuthSvc.Rdb()
	ctx := AuthSvc.Ctx()

	keys, err := rdb.Keys(ctx, "ratelimit:ban:*").Result()
	if err != nil {
		http.Error(w, "Failed to read bans", http.StatusInternalServerError)
		return
	}

	type BanInfo struct {
		ID        string        `json:"id"`
		TTL       time.Duration `json:"ttl"`
		ExpiresAt time.Time     `json:"expires_at"`
	}

	bans := []BanInfo{}
	for _, key := range keys {
		ttl, err := rdb.TTL(ctx, key).Result()
		if err == nil && ttl > 0 {
			id := strings.TrimPrefix(key, "ratelimit:ban:")
			bans = append(bans, BanInfo{
				ID:        id,
				TTL:       ttl,
				ExpiresAt: time.Now().Add(ttl),
			})
		}
	}

	if err := writeJSON(w, http.StatusOK, bans); err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// @Summary Remove a ban for user or IP
// @Tags admin
// @Security BearerAuth
// @Param id path string true "User or IP to unban"
// @Success 204 "Ban removed"
// @Failure 404 {string} string "Not found"
// @Failure 500 {string} string "Redis error"
// @Router /admin/bans/{id} [delete]
func UnbanHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	key := fmt.Sprintf("ratelimit:ban:%s", id)
	rdb := AuthSvc.Rdb()
	ctx := AuthSvc.Ctx()

	ok, err := rdb.Del(ctx, key).Result()
	if err != nil {
		http.Error(w, "Failed to delete ban", http.StatusInternalServerError)
		return
	}
	if ok == 0 {
		http.Error(w, "Ban not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func sessionKey(ip, ua string) string {
	h := sha256.New()
	h.Write([]byte(ip + ua))
	return hex.EncodeToString(h.Sum(nil))
}

func generateRandomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Handle error gracefully — fallback, panic, or log
		log.Printf("Failed to generate random bytes: %v", err)
		return ""
	}
	return hex.EncodeToString(b)
}
