package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

// NewOrderToken creates a one-time secret for guest order access and its storage hash.
func NewOrderToken() (string, string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	return token, orderTokenHash(token), nil
}

// MatchesOrderToken checks a guest token without exposing the stored hash.
func MatchesOrderToken(token, storedHash string) bool {
	if len(token) != 43 || len(storedHash) != 64 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return false
	}
	actual := orderTokenHash(token)
	return subtle.ConstantTimeCompare([]byte(actual), []byte(storedHash)) == 1
}

// orderTokenHash stores only a digest so a database read does not reveal guest tokens.
func orderTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
