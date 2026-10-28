package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/delivery"
	"bongkey/backend/internal/security"
	"github.com/lib/pq"
)

// localCheckoutDB opens only the owner's local PostgreSQL for explicit integration QA.
func localCheckoutDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL checkout QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("checkout integration QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("could not close local PostgreSQL")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal("local PostgreSQL is not reachable with app credentials")
	}
	migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", "005_checkout_idempotency.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal("could not apply local checkout idempotency migration")
	}
	return db
}

// checkoutFixtureProduct inserts one isolated local product and removes it after the test.
func checkoutFixtureProduct(t *testing.T, db *sql.DB, quantity int) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	slug := fmt.Sprintf("qa-stock-%d-%d", os.Getpid(), time.Now().UnixNano())
	var id int
	if err := db.QueryRowContext(ctx, `INSERT INTO products (name, slug, price, stock_quantity, is_active) VALUES ('QA stock fixture', $1, 100, $2, true) RETURNING id`, slug, quantity).Scan(&id); err != nil {
		t.Fatal("could not create local stock fixture")
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM products WHERE id = $1`, id); err != nil {
			t.Error("could not remove local stock fixture")
		}
	})
	return id
}

// TestConcurrentCheckoutStockPostgres proves one of two competing reservations wins.
func TestConcurrentCheckoutStockPostgres(t *testing.T) {
	db := localCheckoutDB(t)
	id := checkoutFixtureProduct(t, db, 1)
	store := PostgresCheckoutStore{DB: db}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for i := 0; i < 2; i++ {
		// The test owns both workers; context bounds database waits and Wait observes completion.
		go func() {
			defer workers.Done()
			<-start
			results <- store.InTransaction(ctx, func(tx CheckoutTx) error {
				_, err := tx.ReserveProduct(ctx, id, 1)
				return err
			})
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	successes, rejected := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrUnavailable):
			rejected++
		default:
			t.Fatalf("unexpected checkout error: %v", err)
		}
	}
	var remaining int
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, id).Scan(&remaining); err != nil {
		t.Fatal("could not read final stock")
	}
	if successes != 1 || rejected != 1 || remaining != 0 {
		t.Fatalf("successes=%d rejected=%d remaining=%d", successes, rejected, remaining)
	}
	forced := errors.New("forced rollback")
	if _, err := db.ExecContext(ctx, `UPDATE products SET stock_quantity = 1 WHERE id = $1`, id); err != nil {
		t.Fatal("could not reset local stock fixture")
	}
	if err := store.InTransaction(ctx, func(tx CheckoutTx) error {
		if _, err := tx.ReserveProduct(ctx, id, 1); err != nil {
			return err
		}
		return forced
	}); !errors.Is(err, forced) {
		t.Fatalf("rollback error = %v, want forced failure", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, id).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("rollback left stock=%d, read error=%v", remaining, err)
	}
}

// TestCheckoutOrderPostgres verifies one accepted order and a rejected repeat use real transactions.
func TestCheckoutOrderPostgres(t *testing.T) {
	db := localCheckoutDB(t)
	id := checkoutFixtureProduct(t, db, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const emailSecret = "synthetic-email-proof-secret-at-least-32-characters"
	proof, err := security.IssueEmailProof(emailSecret, "qa-buyer@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	settings := delivery.Service{Store: delivery.PostgresRepository{DB: db}}
	fee, err := settings.DeliveryFee(ctx, "Dhaka")
	if err != nil {
		t.Fatal("could not load the saved local delivery setting")
	}
	service := CheckoutService{Store: PostgresCheckoutStore{DB: db}, EmailProofSecret: emailSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: settings}
	request := CheckoutRequest{
		IdempotencyKey: "checkout-postgres-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		CustomerEmail: "qa-buyer@example.invalid", EmailProof: proof, ExpectedDeliveryFee: &fee,
		Items: []CheckoutItem{{ProductID: id, Quantity: 1}},
	}
	receipt, err := service.Place(ctx, request)
	if err != nil {
		t.Fatal("local checkout could not persist an order")
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM orders WHERE id = $1`, receipt.ID); err != nil {
			t.Error("could not remove local order fixture")
		}
	})
	var remaining, itemCount, invoiceCount int
	var orderFee, orderTotal, invoiceFee, invoiceTotal int
	var tokenHash string
	var emailVerified bool
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, id).Scan(&remaining); err != nil {
		t.Fatal("could not read checkout stock")
	}
	if err := db.QueryRowContext(ctx, `SELECT guest_token_hash, email_verified FROM orders WHERE id = $1`, receipt.ID).Scan(&tokenHash, &emailVerified); err != nil {
		t.Fatal("could not read local order")
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_items WHERE order_id = $1`, receipt.ID).Scan(&itemCount); err != nil {
		t.Fatal("could not read local order items")
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE order_id = $1`, receipt.ID).Scan(&invoiceCount); err != nil {
		t.Fatal("could not read local invoice")
	}
	if err := db.QueryRowContext(ctx, `SELECT o.delivery_fee, o.total, i.delivery_fee, i.total FROM orders o JOIN invoices i ON i.order_id = o.id WHERE o.id = $1`, receipt.ID).Scan(&orderFee, &orderTotal, &invoiceFee, &invoiceTotal); err != nil {
		t.Fatal("could not read accepted delivery snapshots")
	}
	if remaining != 1 || itemCount != 1 || invoiceCount != 1 || receipt.Total != 100+fee || orderFee != fee || invoiceFee != fee || orderTotal != receipt.Total || invoiceTotal != receipt.Total || !security.MatchesOrderToken(receipt.GuestAccessToken, tokenHash) || !emailVerified {
		t.Fatal("accepted checkout did not persist stock, order, invoice, and private access together")
	}
	changedFee := fee + 1
	request.IdempotencyKey = "checkout-postgres-stale-delivery"
	request.ExpectedDeliveryFee = &changedFee
	if _, err := service.Place(ctx, request); !errors.Is(err, ErrDeliveryFeeChanged) {
		t.Fatal("checkout did not reject a stale delivery estimate")
	}
	request.IdempotencyKey = "checkout-postgres-0001"
	request.ExpectedDeliveryFee = &fee
	retry, err := service.Place(ctx, request)
	if err != nil || retry != receipt {
		t.Fatalf("identical retry did not return the original receipt: retry=%+v err=%v", retry, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, id).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatal("identical retry changed stock")
	}
	// A later admin fee change applies to new orders only; the accepted replay keeps its saved snapshot.
	changedService := service
	changedService.DeliveryFeeProvider = fixedDeliveryFee(fee + 50)
	request.ExpectedDeliveryFee = &changedFee
	if replay, err := changedService.Place(ctx, request); err != nil || replay != receipt {
		t.Fatalf("replay after fee change altered the order: replay=%+v err=%v", replay, err)
	}
	request.ExpectedDeliveryFee = &fee
	access := AccessService{Reader: PostgresReader{DB: db}, EmailProofSecret: emailSecret}
	orderID := fmt.Sprint(receipt.ID)
	order, err := access.GuestInvoice(ctx, orderID, proof)
	if err != nil || order.InvoiceNumber != receipt.InvoiceNumber {
		t.Fatal("verified email could not access its stored invoice summary")
	}
	if len(order.Items) != 1 || order.Items[0].Quantity != 1 || order.Items[0].LensOption != frameOnlyLens || order.Items[0].TotalPrice != receipt.Subtotal {
		t.Fatalf("order detail did not include its item snapshot: %+v", order.Items)
	}
	if byNumber, err := access.Guest(ctx, receipt.OrderNumber, proof); err != nil || byNumber.ID != receipt.ID {
		t.Fatal("order number lookup did not find the same order")
	}
	if _, err := access.Admin(ctx, "0"); !errors.Is(err, ErrNotFound) {
		t.Fatal("non-positive order ID must not match")
	}
	newest, err := (PostgresReader{DB: db}).ListAdmin(ctx, 0)
	if err != nil || len(newest) == 0 || len(newest) > AdminOrderPageSize || newest[0].ID != receipt.ID {
		t.Fatal("newest admin page did not start with the new order")
	}
	older, err := (PostgresReader{DB: db}).ListAdmin(ctx, receipt.ID)
	if err != nil {
		t.Fatal("older admin page failed")
	}
	for _, listed := range older {
		if listed.ID >= receipt.ID {
			t.Fatal("older admin page repeated a newer order")
		}
	}
	otherProof, err := security.IssueEmailProof(emailSecret, "someone-else@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := access.Guest(ctx, orderID, otherProof); !errors.Is(err, ErrNotFound) {
		t.Fatal("another email accessed the invoice summary")
	}
	guestInvoice, err := access.GuestInvoice(ctx, orderID, proof)
	if err != nil || guestInvoice.InvoiceNumber != receipt.InvoiceNumber || guestInvoice.InvoiceIssuedAt == "" || len(guestInvoice.Items) != 1 || guestInvoice.Items[0].UnitPrice != 100 {
		t.Fatalf("verified guest could not read saved invoice lines: %+v, %v", guestInvoice, err)
	}
	if _, err := access.GuestInvoice(ctx, orderID, otherProof); !errors.Is(err, ErrNotFound) {
		t.Fatal("another email accessed saved invoice lines")
	}
	adminDetail, err := access.Admin(ctx, orderID)
	if err != nil || len(adminDetail.Items) != 1 || adminDetail.Items[0].ProductName != "QA stock fixture" || adminDetail.Items[0].UnitPrice != 100 {
		t.Fatalf("admin did not receive saved item snapshots: detail=%+v err=%v", adminDetail, err)
	}
	request.Items[0].Quantity = 2
	request.IdempotencyKey = "checkout-postgres-0002"
	if _, err := service.Place(ctx, request); !errors.Is(err, ErrUnavailable) {
		t.Fatal("checkout accepted more stock than remained")
	}
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, id).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatal("rejected checkout changed stock")
	}
	outsideFee, err := settings.DeliveryFee(ctx, "Chattogram")
	if err != nil {
		t.Fatal("could not load outside-Dhaka fee")
	}
	request.City = "Chattogram"
	request.IdempotencyKey = "checkout-postgres-outside-dhaka"
	request.Items[0].Quantity = 1
	wrongOutsideFee := outsideFee + 1
	request.ExpectedDeliveryFee = &wrongOutsideFee
	if _, err := service.Place(ctx, request); !errors.Is(err, ErrDeliveryFeeChanged) {
		t.Fatal("outside-Dhaka checkout accepted a stale fee")
	}
	request.ExpectedDeliveryFee = &outsideFee
	outsideReceipt, err := service.Place(ctx, request)
	if err != nil || outsideReceipt.DeliveryFee != outsideFee || outsideReceipt.Total != 100+outsideFee {
		t.Fatalf("outside-Dhaka checkout receipt=%+v err=%v", outsideReceipt, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM orders WHERE id = $1`, outsideReceipt.ID); err != nil {
			t.Error("could not remove outside-Dhaka checkout fixture")
		}
	})
	if err := db.QueryRowContext(ctx, `SELECT o.delivery_fee, i.delivery_fee FROM orders o JOIN invoices i ON i.order_id = o.id WHERE o.id = $1`, outsideReceipt.ID).Scan(&orderFee, &invoiceFee); err != nil || orderFee != outsideFee || invoiceFee != outsideFee {
		t.Fatalf("outside-Dhaka order/invoice fee=%d/%d err=%v", orderFee, invoiceFee, err)
	}
}

// TestConcurrentIdempotentCheckoutPostgres proves equal retries create one order and reserve stock once.
func TestConcurrentIdempotentCheckoutPostgres(t *testing.T) {
	db := localCheckoutDB(t)
	productID := checkoutFixtureProduct(t, db, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	proof, err := security.IssueEmailProof(testEmailProofSecret, "qa-buyer@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	service := CheckoutService{Store: PostgresCheckoutStore{DB: db}, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}
	fee := 80
	request := CheckoutRequest{
		IdempotencyKey: fmt.Sprintf("checkout-concurrent-%d-%d", os.Getpid(), time.Now().UnixNano()), CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		CustomerEmail: "qa-buyer@example.invalid", EmailProof: proof, ExpectedDeliveryFee: &fee,
		Items: []CheckoutItem{{ProductID: productID, Quantity: 1}},
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM orders WHERE idempotency_key_hash = $1 AND customer_name = 'QA'`, textHash(request.IdempotencyKey)); err != nil {
			t.Error("could not remove concurrent checkout fixture")
		}
	})
	start := make(chan struct{})
	results := make(chan CheckoutReceipt, 2)
	errorsFound := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for i := 0; i < 2; i++ {
		// The test owns both workers; the timeout bounds lock waits and Wait observes completion.
		go func() {
			defer workers.Done()
			<-start
			receipt, err := service.Place(ctx, request)
			if err != nil {
				errorsFound <- err
				return
			}
			results <- receipt
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errorsFound)
	for err := range errorsFound {
		t.Fatalf("concurrent idempotent checkout failed: %v", err)
	}
	var receipts []CheckoutReceipt
	for receipt := range results {
		receipts = append(receipts, receipt)
	}
	if len(receipts) != 2 || receipts[0] != receipts[1] {
		t.Fatalf("concurrent retries returned different receipts: %+v", receipts)
	}
	var remaining, orderCount, invoiceCount int
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, productID).Scan(&remaining); err != nil {
		t.Fatal("could not read stock after concurrent retry")
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE idempotency_key_hash = $1`, textHash(request.IdempotencyKey)).Scan(&orderCount); err != nil {
		t.Fatal("could not count concurrent retry orders")
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE order_id = $1`, receipts[0].ID).Scan(&invoiceCount); err != nil {
		t.Fatal("could not count concurrent retry invoices")
	}
	if remaining != 1 || orderCount != 1 || invoiceCount != 1 {
		t.Fatalf("remaining=%d orders=%d invoices=%d, want one stock reservation, order and invoice", remaining, orderCount, invoiceCount)
	}
}

// TestNonnegativeStockConstraintPostgres checks the migrated database guard.
func TestNonnegativeStockConstraintPostgres(t *testing.T) {
	db := localCheckoutDB(t)
	id := checkoutFixtureProduct(t, db, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `UPDATE products SET stock_quantity = -1 WHERE id = $1`, id)
	var pgErr *pq.Error
	if !errors.As(err, &pgErr) || string(pgErr.Code) != "23514" {
		t.Fatalf("negative stock should violate CHECK constraint, got %T", err)
	}
}
