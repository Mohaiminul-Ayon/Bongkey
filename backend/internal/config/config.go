package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	AppEnv        string
	ServerPort    string
	DatabaseURL   string
	AdminEmail    string
	AdminPassword string
	JWTSecret     string
	ProjectRoot   string
	EnvFile       string
	EnvFileHint   string
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

	return Config{
		AppEnv:        getEnv("APP_ENV", "development"),
		ServerPort:    getEnv("SERVER_PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://bongkey:bongkey_password@localhost:5432/bongkey?sslmode=disable"),
		AdminEmail:    getEnv("ADMIN_EMAIL", "admin@bongkey.com"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin12345"),
		JWTSecret:     getEnv("JWT_SECRET", "change-this-secret"),
		ProjectRoot:   projectRoot,
		EnvFile:       loadedEnvFile,
		EnvFileHint:   envFileHint,
	}, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

// findProjectRoot locates application sources without requiring Docker configuration.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if fileExists(filepath.Join(dir, "backend", "go.mod")) && fileExists(filepath.Join(dir, "frontend", "package.json")) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root from %s; expected backend/go.mod and frontend/package.json", dir)
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
