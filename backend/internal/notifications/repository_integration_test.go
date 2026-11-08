package notifications

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bongkey/backend/internal/config"

	_ "github.com/lib/pq"
)

// localNotificationDB opens the local PostgreSQL only when explicitly requested, and applies the queue migration.
func localNotificationDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL notification QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("notification QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	t.Cleanup(func() { _ = db.Close() })
	migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", "018_notifications.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatal("could not apply the notification migration")
	}
	return db
}

// TestNotificationQueuePostgres checks the unique event key, claiming, retry timing and the summary query.
func TestNotificationQueuePostgres(t *testing.T) {
	db := localNotificationDB(t)
	store := PostgresStore{DB: db}
	ctx := context.Background()
	key := "qa_notify_test:" + time.Now().Format("150405.000000")
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM notifications WHERE event_key LIKE 'qa_notify_test:%'`) })

	out := Outgoing{Key: key, To: "buyer@example.com", Subject: "QA", Text: "first"}
	if err := store.Enqueue(ctx, out); err != nil {
		t.Fatal(err)
	}
	out.Text = "second"
	if err := store.Enqueue(ctx, out); err != nil {
		t.Fatal(err)
	}
	var count int
	var body string
	if err := db.QueryRow(`SELECT COUNT(*), MIN(body) FROM notifications WHERE event_key = $1`, key).Scan(&count, &body); err != nil || count != 1 || body != "first" {
		t.Fatalf("repeated event was not ignored: count=%d body=%q err=%v", count, body, err)
	}
	if done, err := store.HasEvent(ctx, key); err != nil || !done {
		t.Fatal("queued event not found")
	}

	claimed := claimOwn(t, store, key)
	if claimed == nil || claimed.Attempts != 1 {
		t.Fatalf("queued email was not claimed: %+v", claimed)
	}
	if again := claimOwn(t, store, key); again != nil {
		t.Fatal("an email being sent was claimed twice")
	}
	if err := store.MarkRetry(ctx, claimed.ID, time.Now().Add(time.Hour), "send failed"); err != nil {
		t.Fatal(err)
	}
	if early := claimOwn(t, store, key); early != nil {
		t.Fatal("a retry was claimed before its time")
	}
	if _, err := db.Exec(`UPDATE notifications SET next_attempt_at = NOW() - INTERVAL '1 second' WHERE id = $1`, claimed.ID); err != nil {
		t.Fatal(err)
	}
	retry := claimOwn(t, store, key)
	if retry == nil || retry.Attempts != 2 {
		t.Fatalf("due retry was not claimed with its attempt count: %+v", retry)
	}
	// A row left "sending" by a crash is picked up again once its claim is stale.
	if _, err := db.Exec(`UPDATE notifications SET claimed_at = NOW() - INTERVAL '11 minutes' WHERE id = $1`, retry.ID); err != nil {
		t.Fatal(err)
	}
	if stale := claimOwn(t, store, key); stale == nil || stale.Attempts != 3 {
		t.Fatalf("stale claim was not retried: %+v", stale)
	}
	if err := store.MarkSent(ctx, retry.ID); err != nil {
		t.Fatal(err)
	}
	if sent := claimOwn(t, store, key); sent != nil {
		t.Fatal("a sent email was claimed again")
	}

	day := time.Now().Truncate(24 * time.Hour)
	if _, err := store.Summary(ctx, day, day.Add(24*time.Hour)); err != nil {
		t.Fatalf("summary query failed: %v", err)
	}
}

// claimOwn claims due rows and returns this test's row, releasing any other claimed rows untouched by time.
func claimOwn(t *testing.T, store PostgresStore, key string) *Queued {
	t.Helper()
	queued, err := store.ClaimDue(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	var mine *Queued
	for i := range queued {
		var rowKey string
		if err := store.DB.QueryRow(`SELECT event_key FROM notifications WHERE id = $1`, queued[i].ID).Scan(&rowKey); err != nil {
			t.Fatal(err)
		}
		if rowKey == key {
			mine = &queued[i]
			continue
		}
		// Put other local rows back as they were, so the test never changes unrelated queue state.
		if _, err := store.DB.Exec(`UPDATE notifications SET status = 'pending', attempts = attempts - 1, claimed_at = NULL WHERE id = $1`, queued[i].ID); err != nil {
			t.Fatal(err)
		}
	}
	return mine
}
