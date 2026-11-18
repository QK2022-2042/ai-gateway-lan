package secure

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const apiKeyAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// randomString returns n characters from a URL-safe alphabet.
func randomString(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	for i := range buf {
		buf[i] = apiKeyAlphabet[int(buf[i])%len(apiKeyAlphabet)]
	}
	return string(buf)
}

// NewAPIKey returns (plaintext key, sha256 hash, display prefix).
// The plaintext value is never stored.
func NewAPIKey() (plain, hash, prefix string) {
	plain = "gw-" + randomString(40)
	return plain, HashToken(plain), plain[:10] + "…"
}

// HashToken returns the hex SHA-256 of a token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewSessionToken returns (plaintext, hash).
func NewSessionToken() (string, string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token)
}

// RandomPassword generates a readable but strong password.
func RandomPassword(length int) string {
	if length < 12 {
		length = 16
	}
	const alphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// MaskSecret shows a short, non-sensitive identifier for a stored secret.
func MaskSecret(secret string) string {
	s := strings.TrimSpace(secret)
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "…" + s[len(s)-4:]
}

// BearerToken extracts a bearer token from an Authorization header.
func BearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

var _ = fmt.Sprintf
