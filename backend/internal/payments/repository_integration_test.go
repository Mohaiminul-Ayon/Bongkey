package payments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	_ "github.com/lib/pq"
)

// localDB opens the developer's local PostgreSQL only when RUN_LOCAL_DB_QA=1, applies the payment migrations,
// and refuses any non-local database.
func localDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL payment QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("payment QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	t.Cleanup(func() { db.Close() })
	for _, name := range []string{"011_lens_packages.sql", "012_lens_orders.sql", "013_prescription_review.sql", "014_manual_payments.sql"} {
		migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	return db
}

// TestManualPaymentLifecyclePostgres runs submissions and reviews against real constraints and locks.
func TestManualPaymentLifecyclePostgres(t *testing.T) {
	db := localDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	suffix := fmt.Sprint(time.Now().UnixNano())

	// Keep the owner's real bKash settings and put them back afterwards.
	var oldEnabled bool
	var oldNumber, oldInstructions string
	if err := db.QueryRowContext(ctx, `SELECT enabled, account_number, instructions FROM payment_methods WHERE code = 'bkash'`).Scan(&oldEnabled, &oldNumber, &oldInstructions); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec(`UPDATE payment_methods SET enabled = $1, account_number = $2, instructions = $3 WHERE code = 'bkash'`, oldEnabled, oldNumber, oldInstructions)
	})
	var adminID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO admins (name, email, password_hash) VALUES ('QA', $1, 'x') RETURNING id`, "payment-qa-"+suffix+"@example.invalid").Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	newOrder := func(name string, priceFinal bool) int {
		var id int
		if err := db.QueryRowContext(ctx, `
			INSERT INTO orders (order_number, customer_name, customer_phone, shipping_address, city, subtotal, delivery_fee, total, payment_method, payment_status)
			VALUES ($1, 'QA', '01700000000', 'Local', 'Dhaka', 4920, 80, 5000, 'advance', 'advance_due') RETURNING id`, "PAY-QA-"+name+"-"+suffix).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO order_items (order_id, product_name, quantity, unit_price, total_price, lens_kind, lens_price_final)
			VALUES ($1, 'QA frame', 1, 4920, 4920, 'package', $2)`, id, priceFinal); err != nil {
			t.Fatal(err)
		}
		return id
	}
	orderA, orderB := newOrder("A", true), newOrder("B", false)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM orders WHERE id IN ($1, $2)`, orderA, orderB)
		db.Exec(`DELETE FROM admins WHERE id = $1`, adminID)
	})

	service := Service{Store: PostgresStore{DB: db}}
	if _, err := service.Submit(ctx, orderA, Submission{Method: "bkash", TrxID: "QA0000001", SenderNumber: "01712345678", Amount: 1000}); !errors.Is(err, ErrMethodOff) {
		t.Fatalf("submitted while bKash is off: %v", err)
	}
	if err := service.SaveMethod(ctx, Method{Code: "bkash", Enabled: true, AccountNumber: "01711111111", Instructions: "Send Money"}); err != nil {
		t.Fatal(err)
	}

	// Two identical submissions at the same moment create one payment.
	sub := Submission{Method: "bkash", TrxID: "QA" + suffix[len(suffix)-8:], SenderNumber: "01712345678", Amount: 3000}
	var wg sync.WaitGroup
	results := make([]Payment, 2)
	errs := make([]error, 2)
	for i := range results {
		wg.Add(1)
		// The test owns these two goroutines and waits for both before reading results.
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = service.Submit(ctx, orderA, sub)
		}(i)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || results[0].ID != results[1].ID || results[0].Status != StatusPending {
		t.Fatalf("double submission: %+v %v", results, errs)
	}
	first := results[0]

	// An open lens price blocks paying; more than the order total is refused.
	if _, err := service.Submit(ctx, orderB, sub); !errors.Is(err, ErrPriceNotFinal) {
		t.Fatalf("paid while price open: %v", err)
	}
	if _, err := service.Submit(ctx, orderA, Submission{Method: "bkash", TrxID: "QATOOMUCH1", SenderNumber: "01712345678", Amount: 5001}); !errors.Is(err, ErrInvalidPayment) {
		t.Fatalf("amount above total: %v", err)
	}

	// The same transaction ID on another order is saved but warned about.
	if _, err := db.ExecContext(ctx, `UPDATE order_items SET lens_price_final = true WHERE order_id = $1`, orderB); err != nil {
		t.Fatal(err)
	}
	reused, err := service.Submit(ctx, orderB, sub)
	if err != nil || !slices.Contains(reused.UsedOnOrders, first.OrderNumber) {
		t.Fatalf("reused transaction not flagged: %+v %v", reused, err)
	}

	// The advance is sent in two parts; approving both is allowed and marks the advance received.
	second, err := service.Submit(ctx, orderA, Submission{Method: "bkash", TrxID: "QB" + suffix[len(suffix)-8:], SenderNumber: "01712345678", Amount: 2000})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Approve(ctx, first.ID, adminID); err != nil {
		t.Fatal(err)
	}
	if err := service.Approve(ctx, first.ID, adminID); !errors.Is(err, ErrAlreadyReviewed) {
		t.Fatalf("approved twice: %v", err)
	}
	if err := service.Approve(ctx, second.ID, adminID); err != nil {
		t.Fatalf("second part not approvable: %v", err)
	}
	var paymentStatus string
	if err := db.QueryRowContext(ctx, `SELECT payment_status FROM orders WHERE id = $1`, orderA).Scan(&paymentStatus); err != nil || paymentStatus != "advance_received" {
		t.Fatalf("order payment status = %q %v", paymentStatus, err)
	}
	if _, err := service.Submit(ctx, orderA, Submission{Method: "bkash", TrxID: "QALATE0001", SenderNumber: "01712345678", Amount: 100}); !errors.Is(err, ErrNotPayable) {
		t.Fatalf("submitted after the advance was received: %v", err)
	}

	// A rejected payment keeps its reason, and the customer may send the same ID again after a mistake.
	if err := service.Reject(ctx, reused.ID, adminID, "Not found in bKash"); err != nil {
		t.Fatal(err)
	}
	again, err := service.Submit(ctx, orderB, sub)
	if err != nil || again.ID == reused.ID || again.Status != StatusPending {
		t.Fatalf("resubmission after rejection: %+v %v", again, err)
	}
	list, err := service.ForOrder(ctx, orderB)
	if err != nil || len(list) != 2 || list[0].Status != StatusRejected || list[0].RejectReason != "Not found in bKash" {
		t.Fatalf("order B history: %+v %v", list, err)
	}
}
