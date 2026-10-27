package delivery

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	_ "github.com/lib/pq"
)

// TestDeliverySettingsPostgres verifies writes in a session-local table without changing the owner's setting.
func TestDeliverySettingsPostgres(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL delivery QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("delivery QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	db.SetMaxOpenConns(1) // Keep the temporary table on the same PostgreSQL session.
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("could not close local PostgreSQL")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var existingFee int
	if err := db.QueryRowContext(ctx, `SELECT delivery_fee FROM public.store_settings WHERE id = 1`).Scan(&existingFee); err != nil {
		t.Fatal("saved delivery setting or migration is missing")
	}
	if _, err := db.ExecContext(ctx, `CREATE TEMP TABLE store_settings (LIKE public.store_settings INCLUDING ALL)`); err != nil {
		t.Fatal("could not isolate delivery QA")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO store_settings (id, delivery_fee) VALUES (1, 80)`); err != nil {
		t.Fatal("could not seed isolated delivery QA")
	}
	service := Service{Store: PostgresRepository{DB: db}}
	updated, err := service.Update(ctx, 125, 150)
	if err != nil || updated.DeliveryFee != 125 || updated.OutsideDhakaFee != 150 {
		t.Fatalf("saved delivery charge=%+v, err=%v", updated, err)
	}
	read, err := service.Get(ctx)
	if err != nil || read.DeliveryFee != 125 || read.OutsideDhakaFee != 150 {
		t.Fatalf("read delivery charge=%+v, err=%v", read, err)
	}
	if _, err := service.Update(ctx, -1, 150); !errors.Is(err, ErrInvalidFee) {
		t.Fatal("negative delivery charge was accepted")
	}
	var unchangedFee int
	if err := db.QueryRowContext(ctx, `SELECT delivery_fee FROM public.store_settings WHERE id = 1`).Scan(&unchangedFee); err != nil || unchangedFee != existingFee {
		t.Fatal("owner's delivery setting changed during isolated QA")
	}
}
