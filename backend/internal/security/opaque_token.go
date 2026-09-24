package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

// NewOpaqueToken returns a random bearer credential and its storage-safe digest.
func NewOpaqueToken() (string, string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(value)
	digest, err := OpaqueTokenDigest(token)
	return token, digest, err
}

// OpaqueTokenDigest validates the canonical token format before hashing it.
func OpaqueTokenDigest(token string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != token {
		return "", errors.New("invalid access token")
	}
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:]), nil
}
