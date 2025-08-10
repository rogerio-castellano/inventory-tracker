package ban

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rogerio-castellano/inventory-tracker/internal/redissvc"
)

var (
	alertFrom        = os.Getenv("ALERT_FROM")  // sender email
	alertTo          = os.Getenv("ALERT_TO")    // receiver email
	smtpServer       = os.Getenv("SMTP_SERVER") // smtp.example.com
	smtpPort         = os.Getenv("SMTP_PORT")   // e.g., 587
	smtpUser         = os.Getenv("SMTP_USER")
	smtpPassword     = os.Getenv("SMTP_PASS")
	smtpAuthDisabled = os.Getenv("SMTP_AUTH_DISABLED")

	rdb *redis.Client
	ctx context.Context
)

func SetRedisService(rs *redissvc.RedisService) {
	rdb = rs.Rdb()
	ctx = rs.Ctx()
}

func SendBanAlertEmail(bannedID string, route string, strikes int, r *http.Request) error {
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

	logBanEvent(bannedID, route, int(strikes))

	return nil
}

type BanLogEntry struct {
	Target  string    `json:"target"`
	Route   string    `json:"route"`
	Strikes int       `json:"strikes"`
	Time    time.Time `json:"time"`
}

const DailyBanLogKey = "ratelimit:banlog:daily"

func logBanEvent(target, route string, strikes int) {
	entry := BanLogEntry{
		Target:  target,
		Route:   route,
		Strikes: strikes,
		Time:    time.Now(),
	}
	data, _ := json.Marshal(entry)
	_ = rdb.RPush(ctx, DailyBanLogKey, data).Err()
}

func StartDailyBanSummary(interval time.Duration) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, now.Location())
		if now.After(next) {
			next = next.Add(interval)
		}
		time.Sleep(time.Until(next))
		SendDailyBanSummary()
	}
}

func SendDailyBanSummary() {
	entries, err := rdb.LRange(ctx, DailyBanLogKey, 0, -1).Result()
	if err != nil || len(entries) == 0 {
		return
	}
	_ = rdb.Del(ctx, DailyBanLogKey).Err() // clear after reading

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
