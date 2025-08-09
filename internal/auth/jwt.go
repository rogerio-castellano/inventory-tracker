package auth

import (
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

var jwtSecret = []byte("super-secret-key") // move to env in prod

func GenerateToken(user models.User) (string, error) {
	return buildTokenWithClaims(user, "")
}

func GenerateImpersonationToken(user models.User, impersonator string) (string, error) {
	return buildTokenWithClaims(user, impersonator)
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

func buildTokenWithClaims(user models.User, impersonator string) (string, error) {
	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(15 * time.Minute).Unix(),
	}

	if impersonator != "" {
		claims["impersonator"] = impersonator
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
