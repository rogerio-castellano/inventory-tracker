package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/rogerio-castellano/inventory-tracker/internal/auth"
	"github.com/rogerio-castellano/inventory-tracker/internal/http/handlers"
)

type contextKey string

const userIDKey = contextKey("user_id")

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "Bearer ") {
			http.Error(w, "missing or invalid token", http.StatusUnauthorized)
			return
		}

		token, claims, err := auth.TokenClaims(authorization)
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

func RedisRateLimitMiddleware(route string, maxRequests int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rdb := handlers.AuthSvc.Rdb()
			ctx := handlers.AuthSvc.Ctx()
			key, err := getRateLimitKey(r, route)
			if err != nil {
				http.Error(w, "missing or invalid token", http.StatusUnauthorized)
				return
			}

			pipe := rdb.TxPipeline()
			countCmd := pipe.Incr(ctx, key)
			ttlCmd := pipe.TTL(ctx, key)
			_, err = pipe.Exec(ctx)
			if err != nil {
				http.Error(w, "Rate limit error", http.StatusInternalServerError)
				return
			}

			count := countCmd.Val()
			ttl := ttlCmd.Val()

			// Set expiration if it's a new key
			if count == 1 || ttl < 0 {
				rdb.Expire(ctx, key, window)
				ttl = window
			}

			remaining := max(maxRequests-int(count), 0)

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", int(ttl.Seconds())))

			// If over limit
			if count > int64(maxRequests) {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())))
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getRateLimitKey(r *http.Request, route string) (string, error) {
	authorization := r.Header.Get("Authorization")
	fmt.Println("authorization", authorization)

	if strings.HasPrefix(authorization, "Bearer ") {
		_, claims, err := auth.TokenClaims(authorization)
		if err != nil {
			return "", err
		}

		if _, ok := claims["username"].(string); ok {
			return fmt.Sprintf("ratelimit:%s:%s", route, claims["username"]), nil
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", fmt.Errorf("invalid remote address %v", r.RemoteAddr)
	}

	return fmt.Sprintf("ratelimit:%s:%s", route, host), nil
}
