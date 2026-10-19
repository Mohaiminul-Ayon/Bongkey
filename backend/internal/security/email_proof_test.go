package security

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestEmailProofRejectsWrongScopeAndExpiry keeps email proof separate from admin auth.
func TestEmailProofRejectsWrongScopeAndExpiry(t *testing.T) {
	const secret = "synthetic-email-proof-secret-at-least-32-characters"
	proof, err := IssueEmailProof(secret, "buyer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if email, err := VerifyEmailProof(secret, proof); err != nil || email != "buyer@example.com" {
		t.Fatal("valid email proof was rejected")
	}
	if _, err := VerifyAdminToken(secret, proof); err == nil {
		t.Fatal("email proof was accepted as an admin token")
	}
	if _, err := VerifyEmailProof(secret, proof+"x"); err == nil {
		t.Fatal("altered proof was accepted")
	}
	for _, claims := range []jwt.RegisteredClaims{
		{Issuer: "bongkey-api", Subject: "buyer@example.com", Audience: jwt.ClaimStrings{emailProofAudience}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)), IssuedAt: jwt.NewNumericDate(time.Now().Add(-2 * time.Minute))},
		{Issuer: "bongkey-api", Subject: "buyer@example.com", Audience: jwt.ClaimStrings{"wrong"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)), IssuedAt: jwt.NewNumericDate(time.Now())},
	} {
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyEmailProof(secret, raw); err == nil {
			t.Fatal("expired or wrong-audience proof was accepted")
		}
	}
}

// TestEmailCodeHashSeparatesMailboxes checks that a short code has scoped hashes.
func TestEmailCodeHashSeparatesMailboxes(t *testing.T) {
	const secret = "synthetic-email-proof-secret-at-least-32-characters"
	code, err := NewEmailCode()
	if err != nil || len(code) != 6 {
		t.Fatal("could not create a six-digit code")
	}
	if EmailCodeHash(secret, "a@example.com", code) == EmailCodeHash(secret, "b@example.com", code) {
		t.Fatal("the same code must have different mailbox hashes")
	}
}
