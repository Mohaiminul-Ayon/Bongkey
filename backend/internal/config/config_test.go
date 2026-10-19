package config

import "testing"

// TestValidateSecurityRejectsTemplateSecrets checks fail-closed startup configuration.
func TestValidateSecurityRejectsTemplateSecrets(t *testing.T) {
	for _, secret := range []string{"", "short", "change-this-secret-in-production"} {
		if err := (Config{JWTSecret: secret}).validateSecurity(); err == nil {
			t.Fatal("weak or template JWT secret accepted")
		}
	}
	if err := (Config{JWTSecret: "synthetic-test-secret-at-least-thirty-two-characters"}).validateSecurity(); err != nil {
		t.Fatalf("valid test secret rejected: %v", err)
	}
	for _, cfg := range []Config{
		{JWTSecret: "synthetic-test-secret-at-least-thirty-two-characters", GmailFrom: "sender@gmail.com"},
		{JWTSecret: "synthetic-test-secret-at-least-thirty-two-characters", GmailAppPassword: "synthetic-password"},
	} {
		if err := cfg.validateSecurity(); err == nil {
			t.Fatal("partial Gmail configuration was accepted")
		}
	}
}
