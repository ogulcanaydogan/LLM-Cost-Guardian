package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

const keyPrefix = "lcg_"

// GenerateAPIKey creates a new raw API key plus its prefix and hash.
func GenerateAPIKey() (raw string, prefix string, hash string, err error) {
	buf := make([]byte, 24)
	if _, err = rand.Read(buf); err != nil {
		return "", "", "", fmt.Errorf("generate api key: %w", err)
	}

	raw = keyPrefix + hex.EncodeToString(buf)
	prefix = raw[:12]
	hash = HashAPIKey(raw)
	return raw, prefix, hash, nil
}

// HashAPIKey returns the SHA256 hex digest of an API key.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

// ExtractAPIKey reads the API key from Authorization or X-LCG-API-Key.
func ExtractAPIKey(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-LCG-API-Key")); value != "" {
		return value
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
