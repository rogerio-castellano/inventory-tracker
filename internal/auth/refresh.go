package auth

import (
	"encoding/json"
	"errors"
	"fmt"
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

var tokenStore = map[string]map[string]RefreshTokenEntry{}
var mu sync.Mutex

func GetRefreshToken(username string) (map[string]RefreshTokenEntry, bool, error) {
	tokens, err := GetRefreshTokens()
	if err != nil {
		return map[string]RefreshTokenEntry{}, false, err
	}
	token, ok := tokens[username]
	return token, ok, nil
}

func GetRefreshTokens() (map[string]map[string]RefreshTokenEntry, error) {
	if len(tokenStore) == 0 {
		exists, err := fileExists(refreshTokenFile)
		if err != nil {
			log.Println("Error loading Refresh token file")
		}

		if exists {
			if err := loadRefreshTokens(); err != nil {
				return map[string]map[string]RefreshTokenEntry{}, err
			}
		}
	}

	return tokenStore, nil
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

func SetRefreshToken(username, key string, token RefreshTokenEntry) error {
	mu.Lock()
	defer mu.Unlock()

	token.CreatedAt = time.Now()

	if _, exists := tokenStore[username]; !exists {
		tokenStore[username] = make(map[string]RefreshTokenEntry)
	}
	tokenStore[username][key] = token

	return saveRefreshTokens()
}

func RemoveRefreshToken(username string, key string) error {
	mu.Lock()
	defer mu.Unlock()
	if userTokens, ok := tokenStore[username]; ok {
		delete(userTokens, key)
		if len(tokenStore[username]) == 0 {
			delete(tokenStore, username)
		}
	}

	return saveRefreshTokens()
}

func RemoveUserRefreshTokens(username string) error {
	mu.Lock()
	defer mu.Unlock()
	delete(tokenStore, username)
	return saveRefreshTokens()
}

func loadRefreshTokens() error {
	data, err := os.ReadFile(refreshTokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			tokenStore = make(map[string]map[string]RefreshTokenEntry)
			return nil
		}
		return fmt.Errorf("failed to load refresh token: %w", err)
	}
	return json.Unmarshal(data, &tokenStore)
}

func saveRefreshTokens() error {
	data, err := json.MarshalIndent(tokenStore, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
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

	for username, sessions := range tokenStore {
		for key, entry := range sessions {
			if now.Sub(entry.CreatedAt) > RefreshTokenMaxAge {
				delete(sessions, key)
				changed = true
			}
		}
		if len(sessions) == 0 {
			delete(tokenStore, username)
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
