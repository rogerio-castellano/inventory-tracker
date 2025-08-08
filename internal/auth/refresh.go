package auth

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
	"time"
)

type RefreshTokenEntry struct {
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
}

const refreshTokenFile = "refresh_tokens.json"
const RefreshTokenMaxAge = 7 * 24 * time.Hour // 7 days

var refreshTokenStore = map[string]RefreshTokenEntry{}
var mu sync.Mutex

func GetRefreshToken(key string) (RefreshTokenEntry, bool) {
	token, ok := GetrefreshTokens()[key]
	return token, ok
}

func GetrefreshTokens() map[string]RefreshTokenEntry {
	if len(refreshTokenStore) == 0 {
		exists, err := fileExists(refreshTokenFile)
		if err != nil {
			log.Println("Error loading Refresh token file")
		}

		if exists {
			loadRefreshTokens()
		}
	}

	return refreshTokenStore
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

func SetRefreshToken(key string, token RefreshTokenEntry) {
	mu.Lock()
	token.CreatedAt = time.Now()
	refreshTokenStore[key] = token
	saveRefreshTokens()
	mu.Unlock()
}

func RemoveRefreshToken(key string) error {
	mu.Lock()
	delete(refreshTokenStore, key)
	err := saveRefreshTokens()
	mu.Unlock()
	return err
}

func loadRefreshTokens() error {
	data, err := os.ReadFile(refreshTokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			refreshTokenStore = make(map[string]RefreshTokenEntry)
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &refreshTokenStore)
}

func saveRefreshTokens() error {
	data, err := json.MarshalIndent(refreshTokenStore, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(refreshTokenFile, data, 0600)
}

func StartRefreshTokenCleaner(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		<-ticker.C
		cleanExpiredRefreshTokens()
	}
}

func cleanExpiredRefreshTokens() {
	changed := false
	now := time.Now()

	for username, entry := range refreshTokenStore {
		if now.Sub(entry.CreatedAt) > RefreshTokenMaxAge {
			delete(refreshTokenStore, username)
			changed = true
		}
	}

	if changed {
		err := saveRefreshTokens()
		if err != nil {
			log.Printf("⚠️ Failed to save cleaned refresh tokens: %v", err)
		} else {
			log.Println("🧹 Expired refresh tokens cleaned.")
		}
	}
}
