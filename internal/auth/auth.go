package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const bearerPrefix = "Bearer "

func Authorized(r *http.Request, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}

	token := strings.TrimSpace(r.Header.Get("X-Relay-Secret"))
	if token == "" {
		token = bearerToken(r.Header.Get("Authorization"))
	}
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("secret"))
	}

	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) <= len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(header[len(bearerPrefix):])
}
