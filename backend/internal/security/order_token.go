package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

// IdempotentOrderToken derives the same guest secret for retries without storing it in plaintext.
func IdempotentOrderToken(secret, idempotencyKey string) (string, string, error) {
	if len(secret) < 32 || idempotencyKey == "" {
		return "", "", errors.New("order token secret and idempotency key are required")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("bongkey-order-access\x00"))
	_, _ = mac.Write([]byte(idempotencyKey))
	token := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
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
