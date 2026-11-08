package security

import (
	"crypto/sha256"
	"encoding/hex"
)

// NewSessionToken returns a random browser session token and the SHA-256 hash that is stored instead of it,
// so a database leak does not hand out working sessions.
func NewSessionToken() (token, hash string, err error) {
	token, err = RandomHex(32)
	if err != nil {
		return "", "", err
	}
	return token, SessionTokenHash(token), nil
}

// SessionTokenHash is the stored lookup key for a session token. A fast hash is enough because the token is 256 random bits.
func SessionTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
