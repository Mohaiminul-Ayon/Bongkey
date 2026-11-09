package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	_ "github.com/lib/pq"
)

// TestAuditLogPostgres checks that committed changes are logged, rolled-back ones are not, unwatched columns
// (stock, customer contact) are not, and the log refuses edits.
// Note: the log is append-only, so the fixture's entries stay in the local database by design.
func TestAuditLogPostgres(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL audit QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("audit QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", "022_audit_log.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}

	var productID int
	slug := fmt.Sprintf("qa-audit-%d", time.Now().UnixNano())
	if err := db.QueryRowContext(ctx, `INSERT INTO products (name, slug, price, stock_quantity) VALUES ('QA audit', $1, 100, 1) RETURNING id`, slug).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM products WHERE id = $1`, productID) })
	key := fmt.Sprint(productID)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE products SET price = 999 WHERE id = $1`, productID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE products SET stock_quantity = 5 WHERE id = $1`, productID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE products SET price = 150 WHERE id = $1`, productID); err != nil {
		t.Fatal(err)
	}

	entries, err := Reader{DB: db}.Recent(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var mine []Entry
	for _, entry := range entries {
		if entry.Table == "products" && entry.RecordID == key {
			mine = append(mine, entry)
		}
	}
	if len(mine) != 2 || mine[0].Action != "update" || mine[1].Action != "insert" {
		t.Fatalf("entries for the fixture = %+v; want the committed update and the insert only", mine)
	}
	var changes map[string]struct{ From, To json.RawMessage }
	if err := json.Unmarshal(mine[0].Changes, &changes); err != nil || len(changes) != 1 ||
		string(changes["price"].From) != "100" || string(changes["price"].To) != "150" {
		t.Fatalf("update changes = %s", mine[0].Changes)
	}

	if _, err := db.ExecContext(ctx, `UPDATE audit_log SET changes = '{}' WHERE id = $1`, mine[0].ID); err == nil {
		t.Fatal("an audit entry was edited")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM audit_log WHERE id = $1`, mine[0].ID); err == nil {
		t.Fatal("an audit entry was deleted")
	}
}
