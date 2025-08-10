package http

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"os"
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
				if err := recordRateLimitStrike(key, route, r); err != nil {
					http.Error(w, "Rate limit error", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())))
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

const (
	rateLimitStrikeThreshold = 10
	rateLimitStrikeWindow    = 10 * time.Minute
	banDuration              = 15 * time.Minute
)

func RedisRateLimitPerRole(route string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization := r.Header.Get("Authorization")
			role := "guest"
			if authorization != "" {
				_, claims, err := auth.TokenClaims(authorization)
				if err != nil {
					http.Error(w, "Rate limit error", http.StatusInternalServerError)
					return
				}
				if rRole, ok := claims["role"].(string); ok {
					role = rRole
				}
			}

			cfg := getRateLimitConfigForRole(role)

			key, err := getClientIdentifier(r)
			if err != nil {
				http.Error(w, "Rate limit error", http.StatusInternalServerError)
				return
			}
			redisKey := fmt.Sprintf("ratelimit:%s:%s:%s", route, role, key)

			rdb := handlers.AuthSvc.Rdb()
			ctx := handlers.AuthSvc.Ctx()
			banKey := fmt.Sprintf("ratelimit:ban:%s", key)
			banTTL, err := rdb.TTL(ctx, banKey).Result()

			if err == nil && banTTL > 0 {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(banTTL.Seconds())))
				http.Error(w, "Too many requests — temporarily banned", http.StatusTooManyRequests)
				return
			}

			count, ttl, err := incrementWithTTL(redisKey, cfg.Window)
			if err != nil {
				http.Error(w, "Rate limit error", http.StatusInternalServerError)
				return
			}

			remaining := max(cfg.MaxRequests-int(count), 0)

			// Headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", cfg.MaxRequests))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", int(ttl.Seconds())))

			if count > int64(cfg.MaxRequests) {
				if err := recordRateLimitStrike(redisKey, route, r); err != nil {
					http.Error(w, "Rate limit error", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(ttl.Seconds())))
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func recordRateLimitStrike(key, route string, r *http.Request) error {
	rdb := handlers.AuthSvc.Rdb()
	ctx := handlers.AuthSvc.Ctx()

	strikeKey := fmt.Sprintf("ratelimit:strikes:%s", key)
	strikes, err := rdb.Incr(ctx, strikeKey).Result()
	if err == nil {
		rdb.Expire(ctx, strikeKey, rateLimitStrikeWindow)

		if strikes >= int64(rateLimitStrikeThreshold) {
			key, err := getClientIdentifier(r)
			if err != nil {
				return err
			}

			banKey := fmt.Sprintf("ratelimit:ban:%s", key)
			_ = rdb.Set(ctx, banKey, "1", banDuration).Err()
			log.Printf("🚫 BANNED: %s for %v due to %d+ strikes", banKey, banDuration, strikes)
			sendBanAlertEmail(key, route, int(strikes)) // 📨 trigger alert
		}
	}
	return nil
}

func getRateLimitKey(r *http.Request, route string) (string, error) {
	authorization := r.Header.Get("Authorization")

	if strings.HasPrefix(authorization, "Bearer ") {
		_, claims, err := auth.TokenClaims(authorization)
		if err != nil {
			return "", err
		}

		if username, ok := claims["username"].(string); ok {
			return fmt.Sprintf("ratelimit:%s:%s", route, username), nil
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", fmt.Errorf("invalid remote address %v", r.RemoteAddr)
	}

	return fmt.Sprintf("ratelimit:%s:%s", route, host), nil
}

type RateLimitConfig struct {
	MaxRequests int
	Window      time.Duration
}

func getRateLimitConfigForRole(role string) RateLimitConfig {
	switch role {
	case "admin":
		return RateLimitConfig{MaxRequests: 20, Window: time.Minute}
	case "user":
		return RateLimitConfig{MaxRequests: 10, Window: time.Minute}
	default:
		return RateLimitConfig{MaxRequests: 3, Window: time.Minute} // guests or unknown
	}
}

func getClientIdentifier(r *http.Request) (string, error) {
	authorization := r.Header.Get("Authorization")

	if strings.HasPrefix(authorization, "Bearer ") {
		_, claims, err := auth.TokenClaims(authorization)
		if err != nil {
			return "", err
		}

		if username, ok := claims["username"].(string); ok {
			return username, nil
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", fmt.Errorf("invalid remote address %v", r.RemoteAddr)
	}
	//Fallback
	return host, nil
}

func incrementWithTTL(key string, window time.Duration) (int64, time.Duration, error) {
	rdb := handlers.AuthSvc.Rdb()
	ctx := handlers.AuthSvc.Ctx()

	pipe := rdb.TxPipeline()
	countCmd := pipe.Incr(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, 0, err
	}

	ttl := ttlCmd.Val()
	if countCmd.Val() == 1 || ttl < 0 {
		rdb.Expire(ctx, key, window)
		ttl = window
	}

	return countCmd.Val(), ttl, nil
}

var (
	alertFrom        = os.Getenv("ALERT_FROM")  // sender email
	alertTo          = os.Getenv("ALERT_TO")    // receiver email
	smtpServer       = os.Getenv("SMTP_SERVER") // smtp.example.com
	smtpPort         = os.Getenv("SMTP_PORT")   // e.g., 587
	smtpUser         = os.Getenv("SMTP_USER")
	smtpPassword     = os.Getenv("SMTP_PASS")
	smtpAuthDisabled = os.Getenv("SMTP_AUTH_DISABLED")
)

func sendBanAlertEmail(bannedID string, route string, strikes int) {
	subject := fmt.Sprintf("⚠️ BAN ALERT: %s blocked", bannedID)
	body := fmt.Sprintf("Target: %s\nRoute: %s\nStrikes: %d\nTime: %s", bannedID, route, strikes, time.Now().Format(time.RFC3339))

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", alertFrom, alertTo, subject, body)

	addr := fmt.Sprintf("%s:%s", smtpServer, smtpPort)
	auth := smtp.PlainAuth("", smtpUser, smtpPassword, smtpServer)

	if smtpAuthDisabled != "" {
		auth = nil
	}

	go func() {
		err := smtp.SendMail(addr, auth, alertFrom, []string{alertTo}, []byte(msg))
		if err != nil {
			fmt.Printf("Failed to send alert email: %v\n", err)
		}
	}()
}
