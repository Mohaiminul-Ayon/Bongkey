package config

import (
	"bufio"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	AppEnv           string
	ServerPort       string
	DatabaseURL      string
	AdminEmail       string
	AdminPassword    string
	JWTSecret        string
	GmailFrom        string
	GmailAppPassword string
	// GoogleClientID is the OAuth client for customer "Sign in with Google"; empty turns customer sign-in off.
	GoogleClientID string
	// EmailTestCode turns on local email test mode: addresses ending in @example.test get this fixed
	// 6-digit code and no email is sent. Empty means off; startup refuses it outside development.
	EmailTestCode string
	// MailProvider is "gmail" (SMTP, local work) or "brevo" (HTTPS API, for hosts that block mail ports).
	MailProvider string
	BrevoAPIKey  string
	// MailFrom is the Brevo sender, an address on a domain verified in Brevo; Gmail sends from GmailFrom.
	MailFrom     string
	MailFromName string
	// OwnerNotifyEmail receives the daily summary and payment alerts; empty turns owner emails off.
	OwnerNotifyEmail string
	// SiteURL is the public storefront address used in email links, such as the Track page.
	SiteURL string
	// FrontendOrigins are the browser origins allowed to read API responses (CORS).
	FrontendOrigins []string
	ProjectRoot     string
	// PrescriptionDir is the private folder for prescription files; it is never served as static files.
	PrescriptionDir string
	// ReturnPhotoDir is the private folder for photos attached to return requests; never served as static files.
	ReturnPhotoDir string
	EnvFile        string
	EnvFileHint    string
}

func Load() (Config, error) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return Config{}, err
	}

	envFile := filepath.Join(projectRoot, ".env")
	envFileHint := fmt.Sprintf("Create %s by copying %s.", envFile, filepath.Join(projectRoot, ".env.example"))
	loadedEnvFile, err := loadDotEnv(envFile)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		ServerPort:       getEnv("SERVER_PORT", "8080"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://bongkey:bongkey_password@localhost:5432/bongkey?sslmode=disable"),
		AdminEmail:       getEnv("ADMIN_EMAIL", ""),
		AdminPassword:    getEnv("ADMIN_PASSWORD", ""),
		JWTSecret:        getEnv("JWT_SECRET", ""),
		GmailFrom:        getEnv("GMAIL_FROM", ""),
		GmailAppPassword: getEnv("GMAIL_APP_PASSWORD", ""),
		GoogleClientID:   getEnv("GOOGLE_CLIENT_ID", ""),
		EmailTestCode:    getEnv("EMAIL_TEST_CODE", ""),
		MailProvider:     getEnv("MAIL_PROVIDER", "gmail"),
		BrevoAPIKey:      getEnv("BREVO_API_KEY", ""),
		MailFrom:         getEnv("MAIL_FROM", ""),
		MailFromName:     getEnv("MAIL_FROM_NAME", "Bongkey"),
		OwnerNotifyEmail: getEnv("OWNER_NOTIFY_EMAIL", ""),
		FrontendOrigins:  strings.Split(getEnv("FRONTEND_ORIGIN", "http://localhost:3000"), ","),
		ProjectRoot:      projectRoot,
		PrescriptionDir:  filepath.Join(projectRoot, "backend", "private", "prescriptions"),
		ReturnPhotoDir:   filepath.Join(projectRoot, "backend", "private", "return-photos"),
		EnvFile:          loadedEnvFile,
		EnvFileHint:      envFileHint,
	}
	cfg.SiteURL = strings.TrimRight(getEnv("PUBLIC_SITE_URL", cfg.FrontendOrigins[0]), "/")
	if err := cfg.validateSecurity(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// validateSecurity rejects missing or known template keys before startup.
func (cfg Config) validateSecurity() error {
	if len(cfg.JWTSecret) < 32 || strings.HasPrefix(cfg.JWTSecret, "change-this-secret") {
		return fmt.Errorf("JWT_SECRET must be a non-template secret of at least 32 characters")
	}
	if (cfg.GmailFrom == "") != (cfg.GmailAppPassword == "") {
		return fmt.Errorf("GMAIL_FROM and GMAIL_APP_PASSWORD must be set together")
	}
	switch cfg.MailProvider {
	case "", "gmail":
	case "brevo":
		if cfg.BrevoAPIKey == "" || !plainAddress(cfg.MailFrom) {
			return fmt.Errorf("MAIL_PROVIDER=brevo needs BREVO_API_KEY and a plain MAIL_FROM address")
		}
	default:
		return fmt.Errorf("MAIL_PROVIDER must be gmail or brevo")
	}
	if cfg.OwnerNotifyEmail != "" && !plainAddress(cfg.OwnerNotifyEmail) {
		return fmt.Errorf("OWNER_NOTIFY_EMAIL must be one plain email address")
	}
	// A fixed code must never reach a real shop: fail closed unless this is explicitly a development run.
	if cfg.EmailTestCode != "" && (cfg.AppEnv != "development" || !sixDigits(cfg.EmailTestCode)) {
		return fmt.Errorf("EMAIL_TEST_CODE is only allowed with APP_ENV=development and must be 6 digits")
	}
	return nil
}

// plainAddress accepts one mailbox such as orders@example.com, without a display name.
func plainAddress(value string) bool {
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}

// sixDigits reports whether value is exactly six ASCII digits.
func sixDigits(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if fileExists(filepath.Join(dir, ".env.example")) && fileExists(filepath.Join(dir, "docker-compose.yml")) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root from %s; expected .env.example and docker-compose.yml", dir)
		}
		dir = parent
	}
}

func loadDotEnv(path string) (string, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return "", fmt.Errorf("invalid .env line %d in %s: expected KEY=value", lineNumber, path)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			return "", fmt.Errorf("invalid .env line %d in %s: empty key", lineNumber, path)
		}
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return "", fmt.Errorf("could not set %s from %s: %w", key, path, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("could not parse %s: %w", path, err)
	}

	return path, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
