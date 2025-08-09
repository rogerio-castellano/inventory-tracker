package auth

import (
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

func ParseToken(tokenStr string) (*jwt.Token, error) {
	return jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return jwtSecret, nil
	})
}
