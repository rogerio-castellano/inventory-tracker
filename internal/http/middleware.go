package http

import (
	"context"
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
)

type contextKey string

const userIDKey = contextKey("user_id")

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing or invalid token", http.StatusUnauthorized)
			return
		}

		token, claims, err := handlers.TokenClaims(auth)
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		userID := int(claims["sub"].(float64))

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, err := handlers.GetRoleFromContext(r)
			if err != nil {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}
			if userRole != role {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireRoles(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, err := handlers.GetRoleFromContext(r)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			if slices.Contains(allowedRoles, role) {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
		})
	}
}

func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "Invalid remote address", http.StatusInternalServerError)
			return
		}

		limiter := getVisitor(host)
		if !limiter.Allow() {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
