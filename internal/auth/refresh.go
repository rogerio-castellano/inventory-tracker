package auth

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
)

const refreshTokenFile = "refresh_tokens.json"

var refreshTokenStore = map[string]string{}
var mu sync.Mutex

func RefreshTokens() map[string]string {
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

func SetRefreshToken(key string, value string) {
	mu.Lock()
	refreshTokenStore[key] = value
	saveRefreshTokens()
	mu.Unlock()
}

func loadRefreshTokens() error {
	data, err := os.ReadFile(refreshTokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			refreshTokenStore = map[string]string{}
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
