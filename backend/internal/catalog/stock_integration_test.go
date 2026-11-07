package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	_ "github.com/lib/pq"
)

// TestStockAlertsPostgres checks the admin-edit history label, the low-stock list and the history read.
func TestStockAlertsPostgres(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL stock QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("stock QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", "021_stock_history.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	var categoryID int
	if err := db.QueryRowContext(ctx, `SELECT id FROM categories ORDER BY id LIMIT 1`).Scan(&categoryID); err != nil {
		t.Skip("no local category to attach the fixture to")
	}

	repo := PostgresProductRepository{DB: db}
	input := ProductInput{CategoryID: categoryID, Name: "QA stock alert", Slug: fmt.Sprintf("qa-stock-alert-%d", time.Now().UnixNano()),
		Price: 100, StockQuantity: 3, LowStockThreshold: 2, BrandTier: "budget", FrameType: "full_rim", Gender: "unisex", IsActive: true}
	product, err := repo.Save(ctx, 0, input)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM products WHERE id = $1`, product.ID) })
	if product.LowStockThreshold != 2 {
		t.Fatalf("threshold not saved: %d", product.LowStockThreshold)
	}
	if listed(t, repo, product.ID) {
		t.Fatal("stock 3 above threshold 2 is listed as low")
	}

	expected := 3
	input.StockQuantity, input.ExpectedStockQuantity = 1, &expected
	if _, err := repo.Save(ctx, product.ID, input); err != nil {
		t.Fatal(err)
	}
	if !listed(t, repo, product.ID) {
		t.Fatal("stock 1 at or below threshold 2 is missing from the low list")
	}
	history, err := (StockService{Store: repo}).History(ctx, product.ID)
	if err != nil || len(history) != 2 || history[0].Reason != "admin_edit" || history[0].Change != -2 || history[0].StockAfter != 1 ||
		history[1].Reason != "created" || history[1].Change != 3 {
		t.Fatalf("history = %+v, %v", history, err)
	}
	// A save that keeps the stock writes no history row.
	expected = 1
	if _, err := repo.Save(ctx, product.ID, input); err != nil {
		t.Fatal(err)
	}
	if again, _ := repo.StockHistory(ctx, product.ID); len(again) != 2 {
		t.Fatalf("unchanged stock added history: %+v", again)
	}
}

// listed reports whether a product is on the low-stock list.
func listed(t *testing.T, repo PostgresProductRepository, id int) bool {
	t.Helper()
	items, err := repo.LowStock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
