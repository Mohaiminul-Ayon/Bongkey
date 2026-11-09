package security

import (
	"errors"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const (
	// MinPasswordLength follows current guidance: length matters more than symbol rules.
	MinPasswordLength = 8
	// maxPasswordBytes is bcrypt's input limit; longer input would be silently cut, so it is refused instead.
	maxPasswordBytes = 72
)

// ErrWeakPassword means the password is shorter than 8 characters or longer than bcrypt accepts.
var ErrWeakPassword = errors.New("password must be 8 to 72 characters")

// dummyCustomerHash lets an unknown email cost the same bcrypt work, so timing does not reveal which emails have accounts.
var dummyCustomerHash = sync.OnceValue(func() []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte("bongkey-unknown-customer"), bcrypt.DefaultCost)
	return hash
})

// HashPassword checks the length rule and returns a bcrypt hash for storage.
func HashPassword(password string) (string, error) {
	if len([]rune(password)) < MinPasswordLength || len(password) > maxPasswordBytes {
		return "", ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// CheckPassword reports whether password matches hash. An empty hash (no account, or Google only)
// still runs one bcrypt comparison and returns false.
func CheckPassword(hash, password string) bool {
	if hash == "" || len(password) > maxPasswordBytes {
		_ = bcrypt.CompareHashAndPassword(dummyCustomerHash(), []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
