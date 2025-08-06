package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
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

		token, claims, err := tokenClaims(auth)
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		userID := int(claims["sub"].(float64))

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func tokenClaims(auth string) (*jwt.Token, jwt.MapClaims, error) {
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte("super-secret-key"), nil
	})

	if err != nil || !token.Valid {
		return nil, nil, err
	}
	claims := token.Claims.(jwt.MapClaims)

	return token, claims, nil
}

func GetUserID(r *http.Request) int {
	if val, ok := r.Context().Value(userIDKey).(int); ok {
		return val
	}
	return 0
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, err := getRoleFromContext(r)
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

func getRoleFromContext(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	_, claims, err := tokenClaims(auth)
	if err != nil {
		return "", err
	}

	if role, ok := claims["role"].(string); ok {
		return role, nil
	}
	return "", nil
}

func RequireRoles(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, err := getRoleFromContext(r)
			if err != nil {
				//TODO
			}
			//TODO
			for _, allowed := range allowedRoles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
		})
	}
}
