package emailotp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/security"
	_ "github.com/lib/pq"
)

// TestEmailOTPPostgres proves cooldown, attempt limits, and single-use under concurrency.
func TestEmailOTPPostgres(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL email OTP QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("email OTP integration QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("could not close local PostgreSQL")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal("local PostgreSQL is not reachable with app credentials")
	}
	// The additive migration is applied before exercising repository behavior.
	migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", "004_email_otp.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal("could not apply local email OTP migration")
	}
	const secret = "synthetic-email-proof-secret-at-least-32-characters"
	email := fmt.Sprintf("qa-email-%d-%d@example.invalid", os.Getpid(), time.Now().UnixNano())
	emailKey := security.EmailKey(secret, "email", email)
	ipKey := security.EmailKey(secret, "ip", email)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM email_otp_challenges WHERE email_key = $1`, emailKey); err != nil {
			t.Error("could not remove local email challenge")
		}
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM email_otp_ip_limits WHERE ip_key = $1`, ipKey); err != nil {
			t.Error("could not remove local IP limit")
		}
	})
	store := PostgresStore{DB: db}
	codeHash := security.EmailCodeHash(secret, email, "123456")
	if err := store.Reserve(ctx, emailKey, ipKey, codeHash); err != nil {
		t.Fatal(err)
	}
	if err := store.Reserve(ctx, emailKey, ipKey, codeHash); !errors.Is(err, ErrLimited) {
		t.Fatal("resend cooldown did not reject an immediate request")
	}
	wrongHash := security.EmailCodeHash(secret, email, "654321")
	for i := 0; i < 5; i++ {
		if matched, err := store.Consume(ctx, emailKey, wrongHash); err != nil || matched {
			t.Fatal("wrong code was accepted")
		}
	}
	if matched, err := store.Consume(ctx, emailKey, codeHash); err != nil || matched {
		t.Fatal("code remained valid after five wrong attempts")
	}
	if _, err := db.ExecContext(ctx, `UPDATE email_otp_challenges SET next_request_at = NOW() - INTERVAL '1 second' WHERE email_key = $1`, emailKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Reserve(ctx, emailKey, ipKey, codeHash); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan bool, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for i := 0; i < 2; i++ {
		// The test owns both workers; the context bounds DB waits and Wait observes both results.
		go func() {
			defer workers.Done()
			<-start
			matched, err := store.Consume(ctx, emailKey, codeHash)
			if err != nil {
				t.Error("concurrent consume failed")
			}
			results <- matched
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	winners := 0
	for matched := range results {
		if matched {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent OTP consumed %d times, want 1", winners)
	}
	secondEmailKey := security.EmailKey(secret, "email", email+"-concurrent")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM email_otp_challenges WHERE email_key = $1`, secondEmailKey); err != nil {
			t.Error("could not remove concurrent email challenge")
		}
	})
	reserveResults := make(chan error, 2)
	start = make(chan struct{})
	workers.Add(2)
	for i := 0; i < 2; i++ {
		// The test owns both workers; context bounds DB waits and Wait observes completion.
		go func() {
			defer workers.Done()
			<-start
			reserveResults <- store.Reserve(ctx, secondEmailKey, ipKey, codeHash)
		}()
	}
	close(start)
	workers.Wait()
	close(reserveResults)
	accepted, limited := 0, 0
	for err := range reserveResults {
		switch {
		case err == nil:
			accepted++
		case errors.Is(err, ErrLimited):
			limited++
		default:
			t.Fatalf("concurrent reserve failed: %v", err)
		}
	}
	if accepted != 1 || limited != 1 {
		t.Fatalf("concurrent requests accepted=%d limited=%d, want 1 each", accepted, limited)
	}
	// Move only this test's challenge into the past instead of waiting for wall time.
	if _, err := db.ExecContext(ctx, `UPDATE email_otp_challenges SET expires_at = NOW() - INTERVAL '1 second', consumed_at = NULL, attempts = 0 WHERE email_key = $1`, emailKey); err != nil {
		t.Fatal(err)
	}
	if matched, err := store.Consume(ctx, emailKey, codeHash); err != nil || matched {
		t.Fatal("expired code was accepted")
	}
	if _, err := db.ExecContext(ctx, `UPDATE email_otp_challenges SET next_request_at = NOW() - INTERVAL '1 second', send_count = 3 WHERE email_key = $1`, emailKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Reserve(ctx, emailKey, ipKey, codeHash); !errors.Is(err, ErrLimited) {
		t.Fatal("mailbox hourly limit was not enforced after cooldown")
	}
	if _, err := db.ExecContext(ctx, `UPDATE email_otp_ip_limits SET request_count = 10 WHERE ip_key = $1`, ipKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Reserve(ctx, emailKey, ipKey, codeHash); !errors.Is(err, ErrLimited) {
		t.Fatal("IP hourly limit was not enforced")
	}
}
