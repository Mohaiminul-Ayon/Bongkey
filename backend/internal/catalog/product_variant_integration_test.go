package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/orders"
	_ "github.com/lib/pq"
)

// TestVariantStockPostgres proves unique options and independent transactional stock in local PostgreSQL.
func TestVariantStockPostgres(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL variant QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("variant QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal("local PostgreSQL is not reachable")
	}
	migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", "008_product_variants.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}

	slug := fmt.Sprintf("qa-variant-%d-%d", os.Getpid(), time.Now().UnixNano())
	var rootID int
	if err := db.QueryRowContext(ctx, `INSERT INTO products (name, slug, price, color, size, stock_quantity) VALUES ('QA frame', $1, 100, 'Black', 'M', 2) RETURNING id`, slug).Scan(&rootID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM products WHERE parent_product_id = $1`, rootID); err != nil {
			t.Error(err)
		}
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM products WHERE id = $1`, rootID); err != nil {
			t.Error(err)
		}
	})
	repo := PostgresProductRepository{DB: db}
	variant, err := repo.CreateVariant(ctx, rootID, VariantInput{SKU: fmt.Sprintf("QA-%d", rootID), Color: "Blue", Size: "M", Price: 150, StockQuantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateVariant(ctx, rootID, VariantInput{SKU: fmt.Sprintf("QA-DUP-%d", rootID), Color: "blue", Size: "m", Price: 200, StockQuantity: 2}); !errors.Is(err, ErrDuplicateVariant) {
		t.Fatalf("duplicate color/size error = %v", err)
	}
	if _, err := repo.CreateVariant(ctx, rootID, VariantInput{SKU: variant.SKU, Color: "Red", Size: "L", Price: 200, StockQuantity: 2}); !errors.Is(err, ErrDuplicateVariant) {
		t.Fatalf("duplicate SKU error = %v", err)
	}
	variants, err := repo.ListVariants(ctx, rootID, false)
	if err != nil || len(variants) != 2 {
		t.Fatalf("visible variants=%d error=%v; want base and child", len(variants), err)
	}
	publicProduct, err := (ProductService{Repository: repo}).GetBySlug(ctx, slug)
	if err != nil || len(publicProduct.Variants) != 2 || publicProduct.Variants[1].SKU != variant.SKU {
		t.Fatalf("public variant details=%+v error=%v", publicProduct.Variants, err)
	}
	store := orders.PostgresCheckoutStore{DB: db}
	if err := store.InTransaction(ctx, func(tx orders.CheckoutTx) error {
		reserved, err := tx.ReserveProduct(ctx, variant.ID, 1)
		if err != nil {
			return err
		}
		if reserved.Price != 150 || reserved.SKU != variant.SKU {
			t.Fatalf("reserved variant = %+v", reserved)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var rootStock, variantStock int
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, rootID).Scan(&rootStock); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, variant.ID).Scan(&variantStock); err != nil {
		t.Fatal(err)
	}
	if rootStock != 2 || variantStock != 0 {
		t.Fatalf("root stock=%d variant stock=%d", rootStock, variantStock)
	}
	if err := store.InTransaction(ctx, func(tx orders.CheckoutTx) error { _, err := tx.ReserveProduct(ctx, variant.ID, 1); return err }); !errors.Is(err, orders.ErrUnavailable) {
		t.Fatalf("depleted variant error = %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE products SET stock_quantity = 1 WHERE id = $1`, variant.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE products SET is_active = false WHERE id = $1`, rootID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetBySlug(ctx, variant.Slug); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hidden parent must hide child, got %v", err)
	}
	if err := store.InTransaction(ctx, func(tx orders.CheckoutTx) error { _, err := tx.ReserveProduct(ctx, variant.ID, 1); return err }); !errors.Is(err, orders.ErrUnavailable) {
		t.Fatalf("hidden parent reservation error = %v", err)
	}
}
