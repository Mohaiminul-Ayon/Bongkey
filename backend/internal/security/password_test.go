package security

import (
	"errors"
	"strings"
	"testing"
)

// TestPasswordHashAndCheck covers the length rule, a match, a mismatch and an account without a password.
func TestPasswordHashAndCheck(t *testing.T) {
	for _, weak := range []string{"", "short7!", strings.Repeat("a", 73)} {
		if _, err := HashPassword(weak); !errors.Is(err, ErrWeakPassword) {
			t.Fatalf("HashPassword(%d chars) err = %v, want ErrWeakPassword", len(weak), err)
		}
	}
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "correct horse") {
		t.Fatal("right password refused")
	}
	if CheckPassword(hash, "correct horsE") || CheckPassword("", "correct horse") {
		t.Fatal("wrong password or missing hash accepted")
	}
}
