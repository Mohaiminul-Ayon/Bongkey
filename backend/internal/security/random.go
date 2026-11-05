package security

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomHex returns n unpredictable bytes as hex, for names that must not be guessable.
func RandomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
