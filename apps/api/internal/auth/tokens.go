package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// generateRefreshToken returns a cryptographically random 32-byte token, hex-encoded.
func generateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// hashToken returns the SHA-256 hex digest of token; refresh tokens are stored hashed.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
