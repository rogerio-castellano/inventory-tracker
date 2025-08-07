package handlers

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func GetRoleFromContext(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	_, claims, err := TokenClaims(auth)
	if err != nil {
		return "", err
	}

	if role, ok := claims["role"].(string); ok {
		return role, nil
	}
	return "", nil
}

func TokenClaims(auth string) (*jwt.Token, jwt.MapClaims, error) {
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
