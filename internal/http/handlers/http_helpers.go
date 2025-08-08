package handlers

import (
	"encoding/json"
	"errors"
	"io"
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

type jsonResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// readJSON tries to read the body of a request and converts it into JSON
func readJSON(w http.ResponseWriter, r *http.Request, data any) error {
	maxBytes := 1048576 // one megabyte
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	err := dec.Decode(data)
	if err != nil {
		return err
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must have only a single json value")
	}

	return nil
}

// writeJSON takes a response status code and arbitrary data and writes a json response to the client
func writeJSON(w http.ResponseWriter, status int, data any, headers ...http.Header) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(out)
	if err != nil {
		return err
	}

	return nil
}

// errorJSON takes an error, and optionally a response status code, and generates and sends
// a json error response
func errorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest

	if len(status) > 0 {
		statusCode = status[0]
	}

	var payload jsonResponse
	payload.Error = true
	payload.Message = err.Error()

	return writeJSON(w, statusCode, payload)
}
