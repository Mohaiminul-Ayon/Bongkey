package auth

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	_ "github.com/lib/pq"
)

// TestPostgresLoginLimiterCountsConcurrentAttempts proves the shared window counts every parallel attempt once.
func TestPostgresLoginLimiterCountsConcurrentAttempts(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL login limit QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("login limit integration QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", "010_admin_login_limits.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal("could not apply login limit migration")
	}
	key := fmt.Sprintf("qa-login-limit-%d", time.Now().UnixNano())
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM admin_login_limits WHERE client_key = $1`, key) })

	limiter := PostgresLoginLimiter{DB: db}
	var wg sync.WaitGroup
	errs := make(chan error, loginAttemptsPerWindow)
	// The test owns these workers; each sends at most one error and exits, and Wait bounds their lifetime.
	for i := 0; i < loginAttemptsPerWindow; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := limiter.Count(ctx, key); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent count failed: %v", err)
	}
	attempts, err := limiter.Count(ctx, key)
	if err != nil || attempts != loginAttemptsPerWindow+1 {
		t.Fatalf("attempts = %d, err = %v; want %d", attempts, err, loginAttemptsPerWindow+1)
	}
}
