package callbacks

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	_ "github.com/lib/pq"
)

// TestCallbackRequestsPostgres saves, lists and closes a request against the real table and its checks,
// only on the developer's local database (RUN_LOCAL_DB_QA=1).
func TestCallbackRequestsPostgres(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL call-back QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("call-back QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	t.Cleanup(func() { db.Close() })
	migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", "024_callback_requests.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store := PostgresStore{DB: db}
	saved, err := store.Create(ctx, Request{Name: "QA call-back", Phone: "01712345678", ProductSlug: "qa-frame", Message: "QA progressive lens"})
	if err != nil || saved.ID == 0 || saved.Status != StatusOpen || saved.HandledAt != nil {
		t.Fatalf("saved = %+v %v", saved, err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM callback_requests WHERE id = $1`, saved.ID) })

	// The table refuses what the service would refuse, in case another writer skips the service.
	if _, err := store.Create(ctx, Request{Name: "QA", Phone: "0212345678", Message: "QA lens"}); err == nil {
		t.Fatal("landline phone accepted by the table")
	}

	open, err := store.List(ctx, StatusOpen, 50)
	if err != nil || !contains(open, saved.ID) {
		t.Fatalf("open list = %d rows %v", len(open), err)
	}
	if err := store.MarkDone(ctx, saved.ID); err != nil {
		t.Fatal(err)
	}
	var first time.Time
	if err := db.QueryRowContext(ctx, `SELECT handled_at FROM callback_requests WHERE id = $1`, saved.ID).Scan(&first); err != nil {
		t.Fatal(err)
	}
	// Marking it done again is a no-op that keeps the first handled time.
	if err := store.MarkDone(ctx, saved.ID); err != nil {
		t.Fatal(err)
	}
	var again time.Time
	if err := db.QueryRowContext(ctx, `SELECT handled_at FROM callback_requests WHERE id = $1`, saved.ID).Scan(&again); err != nil || !again.Equal(first) {
		t.Fatalf("handled time changed: %v -> %v %v", first, again, err)
	}
	done, err := store.List(ctx, StatusDone, 50)
	if err != nil || !contains(done, saved.ID) {
		t.Fatalf("done list = %d rows %v", len(done), err)
	}
	if err := store.MarkDone(ctx, -1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing request: %v", err)
	}
}

// contains reports whether the list has a request with this ID.
func contains(list []Request, id int64) bool {
	for _, r := range list {
		if r.ID == id {
			return true
		}
	}
	return false
}
