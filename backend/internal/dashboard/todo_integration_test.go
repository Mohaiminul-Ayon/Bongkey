package dashboard

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	_ "github.com/lib/pq"
)

// TestTodoCountsPostgres runs the count query against the real tables and checks that a new call-back
// raises its count by one, only on the developer's local database (RUN_LOCAL_DB_QA=1).
func TestTodoCountsPostgres(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL to-do QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("to-do QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	reader := Reader{DB: db}
	before, err := reader.Todo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := db.QueryRowContext(ctx, `INSERT INTO callback_requests (name, phone, message) VALUES ('QA Todo', '01700000000', 'to-do count check') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM callback_requests WHERE id = $1`, id) })
	after, err := reader.Todo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.CallbacksOpen != before.CallbacksOpen+1 {
		t.Fatalf("call-backs before %d, after %d", before.CallbacksOpen, after.CallbacksOpen)
	}
}
