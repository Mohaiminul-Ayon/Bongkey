package security

import (
	"strings"
	"testing"
)

// TestOpaqueToken verifies credential format and that the persisted value is a digest.
func TestOpaqueToken(t *testing.T) {
	token, digest, err := NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 43 || len(digest) != 64 || strings.Contains(digest, token) {
		t.Fatal("unsafe token representation")
	}
	got, err := OpaqueTokenDigest(token)
	if err != nil || got != digest {
		t.Fatal("digest does not round-trip")
	}
	for _, invalid := range []string{"", "admin", strings.Repeat("x", 42), token + "=", strings.Repeat("!", 43)} {
		if _, err := OpaqueTokenDigest(invalid); err == nil {
			t.Fatal("accepted malformed credential")
		}
	}
}
