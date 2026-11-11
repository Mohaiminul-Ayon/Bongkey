package returns

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
	_ "github.com/lib/pq"
)

// TestReturnTicketsPostgres runs a ticket through the real table, its one-open-ticket index and its refund
// checks, only on the developer's local database (RUN_LOCAL_DB_QA=1).
func TestReturnTicketsPostgres(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL return QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("return QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	t.Cleanup(func() { db.Close() })
	for _, name := range []string{"022_audit_log.sql", "025_return_requests.sql"} {
		migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var orderID int
	if err := db.QueryRowContext(ctx, `
		INSERT INTO orders (order_number, customer_name, customer_phone, shipping_address, city, subtotal, delivery_fee, total, order_status, shipped_at, delivered_at)
		VALUES ($1, 'QA Return', '01700000000', 'Local', 'Dhaka', 2920, 80, 3000, 'delivered', NOW() - INTERVAL '1 day', NOW()) RETURNING id`,
		fmt.Sprintf("RET-QA-%d", time.Now().UnixNano())).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM orders WHERE id = $1`, orderID) })

	store := PostgresStore{DB: db}
	service := Service{Store: store, Photos: nil}
	exchange, err := store.Create(ctx, orderID, KindExchange, "QA want the black one", nil)
	if err != nil || exchange.Status != StatusOpen || exchange.OrderTotal != 3000 || exchange.DeliveryFee != 80 {
		t.Fatalf("exchange = %+v %v", exchange, err)
	}
	// A second open ticket on the same order is refused by the index, even from another device.
	if _, err := store.Create(ctx, orderID, KindDefect, "QA hinge broke", nil); !errors.Is(err, ErrOpenTicket) {
		t.Fatalf("second open ticket: %v", err)
	}
	// The table refuses a refund on an exchange even if the service check were skipped.
	if err := store.RecordRefund(ctx, exchange.ID, Refund{Amount: 100, Method: "cash", PaidAt: time.Now()}, ""); !errors.Is(err, ErrNotRefundable) {
		t.Fatalf("refund on an exchange: %v", err)
	}
	if err := service.Update(ctx, exchange.ID, StatusClosed, OutcomeExchanged, "QA swapped for black"); err != nil {
		t.Fatal(err)
	}
	// A closed ticket stays as it ended.
	if err := service.Update(ctx, exchange.ID, StatusOpen, "", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reopened a closed ticket: %v", err)
	}

	defect, err := store.Create(ctx, orderID, KindDefect, "QA hinge broke", []string{"0123456789abcdef0123456789abcdef.jpg"})
	if err != nil || defect.PhotoCount != 1 {
		t.Fatalf("defect after the exchange closed = %+v %v", defect, err)
	}
	if err := service.RecordRefund(ctx, defect.ID, Refund{Amount: 2840, Method: "bkash", Reference: "QA9ABC12XY"}, "QA frame refunded minus delivery"); err != nil {
		t.Fatal(err)
	}
	got, err := service.Get(ctx, defect.ID)
	if err != nil || got.Status != StatusClosed || got.Outcome != OutcomeRefunded || got.RefundAmount == nil || *got.RefundAmount != 2840 ||
		got.RefundedAt == nil || got.ClosedAt == nil || got.RefundDue == nil || got.Overdue {
		t.Fatalf("refunded ticket = %+v %v", got, err)
	}
	if err := service.RecordRefund(ctx, defect.ID, Refund{Amount: 1, Method: "cash"}, ""); !errors.Is(err, ErrNotRefundable) {
		t.Fatalf("second refund: %v", err)
	}
	var audited int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log WHERE table_name = 'return_requests' AND record_id IN ($1, $2)`,
		fmt.Sprint(exchange.ID), fmt.Sprint(defect.ID)).Scan(&audited); err != nil || audited < 2 {
		t.Fatalf("audit rows = %d %v", audited, err)
	}
	list, err := service.List(ctx, StatusClosed)
	if err != nil || len(list) < 2 {
		t.Fatalf("closed list = %d %v", len(list), err)
	}
}
