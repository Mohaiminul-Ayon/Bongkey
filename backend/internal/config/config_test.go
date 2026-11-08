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

// TestMailProviderSettings fails closed on an unknown provider or an incomplete Brevo setup.
func TestMailProviderSettings(t *testing.T) {
	const secret = "synthetic-test-secret-at-least-thirty-two-characters"
	for name, tc := range map[string]struct {
		cfg Config
		ok  bool
	}{
		"gmail default":       {Config{}, true},
		"brevo complete":      {Config{MailProvider: "brevo", BrevoAPIKey: "synthetic-key", MailFrom: "orders@example.com"}, true},
		"brevo without key":   {Config{MailProvider: "brevo", MailFrom: "orders@example.com"}, false},
		"brevo without from":  {Config{MailProvider: "brevo", BrevoAPIKey: "synthetic-key"}, false},
		"brevo display name":  {Config{MailProvider: "brevo", BrevoAPIKey: "synthetic-key", MailFrom: "Shop <orders@example.com>"}, false},
		"unknown provider":    {Config{MailProvider: "sendmail"}, false},
		"owner email valid":   {Config{OwnerNotifyEmail: "owner@example.com"}, true},
		"owner email invalid": {Config{OwnerNotifyEmail: "owner@"}, false},
	} {
		tc.cfg.JWTSecret = secret
		if err := tc.cfg.validateSecurity(); (err == nil) != tc.ok {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

// TestEmailTestCodeOnlyInDevelopment keeps the fixed email code out of any non-development run.
func TestEmailTestCodeOnlyInDevelopment(t *testing.T) {
	const secret = "synthetic-test-secret-at-least-thirty-two-characters"
	for name, tc := range map[string]struct {
		env, code string
		ok        bool
	}{
		"off in production":     {"production", "", true},
		"development, 6 digits": {"development", "123456", true},
		"production refused":    {"production", "123456", false},
		"staging refused":       {"staging", "123456", false},
		"empty env refused":     {"", "123456", false},
		"too short":             {"development", "1234", false},
		"not digits":            {"development", "12a456", false},
	} {
		err := (Config{JWTSecret: secret, AppEnv: tc.env, EmailTestCode: tc.code}).validateSecurity()
		if (err == nil) != tc.ok {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}
