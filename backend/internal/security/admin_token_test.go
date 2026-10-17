package security

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testAdminSecret = "synthetic-test-secret-at-least-thirty-two-characters"

// TestAdminTokenVerification covers signed sessions and invalid claim/algorithm paths.
func TestAdminTokenVerification(t *testing.T) {
	valid, err := IssueAdminToken(testAdminSecret, 7)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := VerifyAdminToken(testAdminSecret, valid); err != nil || id != 7 {
		t.Fatalf("valid token: id=%d err=%v", id, err)
	}

	now := time.Now()
	base := adminClaims{Role: "owner", RegisteredClaims: jwt.RegisteredClaims{
		Issuer: adminIssuer, Subject: "7", Audience: jwt.ClaimStrings{adminAudience},
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}}
	tests := []struct {
		name   string
		change func(*adminClaims)
		method jwt.SigningMethod
		secret string
	}{
		{"expired", func(c *adminClaims) { c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Hour)) }, jwt.SigningMethodHS256, testAdminSecret},
		{"missing expiration", func(c *adminClaims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256, testAdminSecret},
		{"wrong role", func(c *adminClaims) { c.Role = "customer" }, jwt.SigningMethodHS256, testAdminSecret},
		{"missing role", func(c *adminClaims) { c.Role = "" }, jwt.SigningMethodHS256, testAdminSecret},
		{"wrong issuer", func(c *adminClaims) { c.Issuer = "elsewhere" }, jwt.SigningMethodHS256, testAdminSecret},
		{"missing audience", func(c *adminClaims) { c.Audience = nil }, jwt.SigningMethodHS256, testAdminSecret},
		{"missing subject", func(c *adminClaims) { c.Subject = "" }, jwt.SigningMethodHS256, testAdminSecret},
		{"invalid subject", func(c *adminClaims) { c.Subject = "0" }, jwt.SigningMethodHS256, testAdminSecret},
		{"missing issued at", func(c *adminClaims) { c.IssuedAt = nil }, jwt.SigningMethodHS256, testAdminSecret},
		{"wrong algorithm", func(*adminClaims) {}, jwt.SigningMethodHS384, testAdminSecret},
		{"wrong secret", func(*adminClaims) {}, jwt.SigningMethodHS256, "different-test-secret-at-least-thirty-two-characters"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claims := base
			tc.change(&claims)
			raw, err := jwt.NewWithClaims(tc.method, claims).SignedString([]byte(tc.secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyAdminToken(testAdminSecret, raw); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	if _, err := VerifyAdminToken(testAdminSecret, "malformed"); err == nil {
		t.Fatal("malformed token accepted")
	}
	if _, err := IssueAdminToken("short", 7); err == nil {
		t.Fatal("weak signing secret accepted")
	}
}
