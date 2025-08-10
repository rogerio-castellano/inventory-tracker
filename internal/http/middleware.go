package http

import (
	"context"
	"encoding/json"
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
			if err := sendBanAlertEmail(key, route, int(strikes), r); err != nil { // 📨 trigger alert
				return err
			}
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

func sendBanAlertEmail(bannedID string, route string, strikes int, r *http.Request) error {
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
			log.Printf("Failed to send alert email: %v\n", err)
		}
	}()

	key, err := getClientIdentifier(r)
	if err != nil {
		return err
	}
	logBanEvent(key, route, int(strikes))

	return nil
}

type BanLogEntry struct {
	Target  string    `json:"target"`
	Route   string    `json:"route"`
	Strikes int       `json:"strikes"`
	Time    time.Time `json:"time"`
}

const dailyBanLogKey = "ratelimit:banlog:daily"

func logBanEvent(target, route string, strikes int) {
	rdb := handlers.AuthSvc.Rdb()
	ctx := handlers.AuthSvc.Ctx()

	entry := BanLogEntry{
		Target:  target,
		Route:   route,
		Strikes: strikes,
		Time:    time.Now(),
	}
	data, _ := json.Marshal(entry)
	_ = rdb.RPush(ctx, dailyBanLogKey, data).Err()
}

func StartDailyBanSummary(interval time.Duration) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, now.Location())
		if now.After(next) {
			next = next.Add(interval)
		}
		time.Sleep(time.Until(next))
		sendDailyBanSummary()
	}
}

func sendDailyBanSummary() {
	rdb := handlers.AuthSvc.Rdb()
	ctx := handlers.AuthSvc.Ctx()

	entries, err := rdb.LRange(ctx, dailyBanLogKey, 0, -1).Result()
	if err != nil || len(entries) == 0 {
		return
	}
	_ = rdb.Del(ctx, dailyBanLogKey).Err() // clear after reading

	// parse logs and aggregate
	var logs []BanLogEntry
	routeCounts := make(map[string]int)
	targetCounts := make(map[string]int)

	for _, item := range entries {
		var entry BanLogEntry
		if err := json.Unmarshal([]byte(item), &entry); err == nil {
			logs = append(logs, entry)
			routeCounts[entry.Route]++
			targetCounts[entry.Target]++
		}
	}

	// compose HTML
	var sb strings.Builder
	sb.WriteString("<h2>📊 Daily Ban Summary</h2>")
	sb.WriteString(fmt.Sprintf("<p>Total bans: <strong>%d</strong></p>", len(logs)))

	// 🔢 Summary by route
	sb.WriteString("<h3>🚪 By Route</h3><ul>")
	for route, count := range routeCounts {
		sb.WriteString(fmt.Sprintf("<li><code>%s</code>: %d</li>", route, count))
	}
	sb.WriteString("</ul>")

	// 👤 Summary by user/IP
	sb.WriteString("<h3>👤 By User/IP</h3><ul>")
	for target, count := range targetCounts {
		sb.WriteString(fmt.Sprintf("<li>%s: %d</li>", target, count))
	}
	sb.WriteString("</ul>")

	// 📜 Full list
	sb.WriteString("<h3>📋 Full Log</h3><ul>")
	for _, entry := range logs {
		sb.WriteString(fmt.Sprintf("<li><b>%s</b> on <code>%s</code> (%d strikes) at %s</li>",
			entry.Target, entry.Route, entry.Strikes, entry.Time.Format(time.RFC822)))
	}
	sb.WriteString("</ul>")
	subject := "📊 Daily Ban Report"

	msg := strings.Join([]string{
		"From: " + alertFrom,
		"To: " + alertTo,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=\"UTF-8\"",
		"",
		sb.String(),
	}, "\r\n")

	addr := fmt.Sprintf("%s:%s", smtpServer, smtpPort)
	auth := smtp.PlainAuth("", smtpUser, smtpPassword, smtpServer)

	if smtpAuthDisabled != "" {
		auth = nil
	}

	go func() {
		err = smtp.SendMail(addr, auth, alertFrom, []string{alertTo}, []byte(msg))
		if err != nil {
			log.Printf("❌ Failed to send email: %v\n", err)
		} else {
			log.Println("📬 Daily ban summary sent via SMTP.")
		}
	}()
}
