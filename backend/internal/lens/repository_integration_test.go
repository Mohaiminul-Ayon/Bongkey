package lens

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
)

// TestPackagesPostgres checks category fit, disabled packages and constraint mapping on local PostgreSQL.
func TestPackagesPostgres(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL lens QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("lens QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", "011_lens_packages.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal("could not apply lens migration")
	}

	suffix := fmt.Sprint(time.Now().UnixNano())
	var fitsID, otherID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO categories (name, slug) VALUES ($1, $1) RETURNING id`, "lens-qa-fits-"+suffix).Scan(&fitsID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO categories (name, slug) VALUES ($1, $1) RETURNING id`, "lens-qa-other-"+suffix).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	frame := "lens-qa-frame-" + suffix
	if _, err := db.ExecContext(ctx, `INSERT INTO products (category_id, name, slug, sku, price) VALUES ($1, $2, $2, $2, 1000)`, fitsID, frame); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		db.ExecContext(cleanup, `DELETE FROM lens_packages WHERE name LIKE $1`, "%"+suffix)
		db.ExecContext(cleanup, `DELETE FROM products WHERE slug = $1`, frame)
		db.ExecContext(cleanup, `DELETE FROM categories WHERE id IN ($1, $2)`, fitsID, otherID)
	})

	service := Service{Repository: PostgresRepository{DB: db}}
	for _, pkg := range []Package{
		{Name: "Fits " + suffix, Price: 1500, Active: true, CategoryIDs: []int64{fitsID}},
		{Name: "Other " + suffix, Price: 2500, Active: true, CategoryIDs: []int64{otherID}},
		{Name: "Off " + suffix, Price: 3500, Active: false, CategoryIDs: []int64{fitsID}},
	} {
		if err := service.Save(ctx, 0, pkg); err != nil {
			t.Fatalf("save %s: %v", pkg.Name, err)
		}
	}
	shown, err := service.ForProduct(ctx, frame)
	if err != nil || len(shown) != 1 || shown[0].Name != "Fits "+suffix || shown[0].Price != 1500 {
		t.Fatalf("frame should see only the enabled package of its category: %+v, err %v", shown, err)
	}
	if err := service.Save(ctx, 0, Package{Name: "Fits " + suffix, Price: 900}); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("duplicate name err = %v", err)
	}
	if err := service.Save(ctx, 0, Package{Name: "Ghost " + suffix, Price: 900, CategoryIDs: []int64{1 << 30}}); !errors.Is(err, ErrInvalidPackage) {
		t.Fatalf("unknown category err = %v", err)
	}
	if err := service.Save(ctx, 1<<30, Package{Name: "Missing " + suffix, Price: 900}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing package err = %v", err)
	}
	all, err := service.ListAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range all {
		if pkg.Name == "Fits "+suffix {
			// Moving the package to the other category removes it from this frame in one save.
			pkg.CategoryIDs = []int64{otherID}
			if err := service.Save(ctx, pkg.ID, pkg); err != nil {
				t.Fatal(err)
			}
		}
	}
	if shown, err := service.ForProduct(ctx, frame); err != nil || len(shown) != 0 {
		t.Fatalf("frame still sees a moved package: %+v, err %v", shown, err)
	}
}
