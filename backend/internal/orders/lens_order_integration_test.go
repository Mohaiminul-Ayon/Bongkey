package orders

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/delivery"
	"bongkey/backend/internal/lens"
	"bongkey/backend/internal/security"
)

// TestLensOrderLifecyclePostgres runs a lens order from checkout to confirmation on local PostgreSQL:
// the price above the power limit stays open, totals follow the owner's price, and confirmation waits for every step.
func TestLensOrderLifecyclePostgres(t *testing.T) {
	db := localCheckoutDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"011_lens_packages.sql", "012_lens_orders.sql"} {
		migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(migration)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	var categoryID, productID, packageID int
	if err := db.QueryRowContext(ctx, `INSERT INTO categories (name, slug) VALUES ($1, $1) RETURNING id`, "lens-order-qa-"+suffix).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO products (category_id, name, slug, price, stock_quantity, is_active) VALUES ($1, 'QA lens frame', $2, 1000, 3, true) RETURNING id`, categoryID, "lens-order-qa-"+suffix).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO lens_packages (name, price, max_power) VALUES ($1, 2500, 600) RETURNING id`, "QA blue-cut "+suffix).Scan(&packageID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO lens_package_categories (package_id, category_id) VALUES ($1, $2)`, packageID, categoryID); err != nil {
		t.Fatal(err)
	}
	var orderID int
	t.Cleanup(func() {
		cleanup := context.Background()
		db.ExecContext(cleanup, `DELETE FROM orders WHERE id = $1`, orderID)
		db.ExecContext(cleanup, `DELETE FROM lens_packages WHERE id = $1`, packageID)
		db.ExecContext(cleanup, `DELETE FROM products WHERE id = $1`, productID)
		db.ExecContext(cleanup, `DELETE FROM categories WHERE id = $1`, categoryID)
	})

	const emailSecret = "synthetic-email-proof-secret-at-least-32-characters"
	proof, err := security.IssueEmailProof(emailSecret, "qa-lens@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	settings := delivery.Service{Store: delivery.PostgresRepository{DB: db}}
	fee, err := settings.DeliveryFee(ctx, "Dhaka")
	if err != nil {
		t.Fatal(err)
	}
	files := lens.PrescriptionStore{Dir: t.TempDir()}
	checkout := CheckoutService{Store: PostgresCheckoutStore{DB: db}, EmailProofSecret: emailSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: settings, Prescriptions: files}
	high := -1600
	receipt, err := checkout.Place(ctx, CheckoutRequest{
		IdempotencyKey: "lens-order-qa-" + suffix, CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		CustomerEmail: "qa-lens@example.invalid", EmailProof: proof, ExpectedDeliveryFee: &fee,
		Items: []CheckoutItem{{ProductID: productID, Quantity: 1, LensKind: LensKindPackage, LensPackageID: packageID, LensPower: LensWithPower, SPHRight: &high, Prescription: []byte("%PDF-1.7 qa")}},
	})
	if err != nil {
		t.Fatalf("lens checkout: %v", err)
	}
	orderID = receipt.ID
	if receipt.Total != 1000+2500+fee || receipt.PaymentMethod != "advance" {
		t.Fatalf("receipt = %+v", receipt)
	}

	access := AccessService{Reader: PostgresReader{DB: db}}
	order, err := access.Admin(ctx, fmt.Sprint(orderID))
	if err != nil || len(order.Items) != 1 {
		t.Fatalf("admin order: %+v %v", order, err)
	}
	item := order.Items[0]
	if item.LensPriceFinal || item.PrescriptionStatus != "pending" || !item.HasPrescription || item.SPHRight == nil || *item.SPHRight != -1600 || item.SPHLeft != nil {
		t.Fatalf("lens line snapshot = %+v", item)
	}

	fulfillment := FulfillmentService{Store: PostgresFulfillmentStore{DB: db}}
	review := LensReviewService{Store: PostgresLensReviewStore{DB: db}}
	if _, err := fulfillment.ChangeStatus(ctx, orderID, "confirmed"); !errors.Is(err, ErrLensPriceNotFinal) {
		t.Fatalf("confirmed with open price: %v", err)
	}
	if err := review.MarkAdvanceReceived(ctx, orderID); !errors.Is(err, ErrLensPriceNotFinal) {
		t.Fatalf("advance taken before the price was final: %v", err)
	}
	if err := review.SetLensPrice(ctx, orderID, item.ID, 4200, ""); err != nil {
		t.Fatal(err)
	}
	var orderTotal, invoiceTotal int
	if err := db.QueryRowContext(ctx, `SELECT o.total, i.total FROM orders o JOIN invoices i ON i.order_id = o.id WHERE o.id = $1`, orderID).Scan(&orderTotal, &invoiceTotal); err != nil {
		t.Fatal(err)
	}
	if orderTotal != 1000+4200+fee || invoiceTotal != orderTotal {
		t.Fatalf("totals after price change: order=%d invoice=%d", orderTotal, invoiceTotal)
	}
	if _, err := fulfillment.ChangeStatus(ctx, orderID, "confirmed"); !errors.Is(err, ErrPrescriptionNotAccepted) {
		t.Fatalf("confirmed with unchecked prescription: %v", err)
	}
	if err := review.SetPrescriptionStatus(ctx, orderID, item.ID, "accepted"); err != nil {
		t.Fatal(err)
	}
	if _, err := fulfillment.ChangeStatus(ctx, orderID, "confirmed"); !errors.Is(err, ErrAdvanceNotReceived) {
		t.Fatalf("confirmed without advance: %v", err)
	}
	if err := review.MarkAdvanceReceived(ctx, orderID); err != nil {
		t.Fatal(err)
	}
	if err := review.SetLensPrice(ctx, orderID, item.ID, 100, ""); !errors.Is(err, ErrLensReviewClosed) {
		t.Fatalf("price changed after the advance was paid: %v", err)
	}
	if status, err := fulfillment.ChangeStatus(ctx, orderID, "confirmed"); err != nil || status != "confirmed" {
		t.Fatalf("ready lens order not confirmed: %s %v", status, err)
	}
	if err := review.SetPrescriptionStatus(ctx, orderID, item.ID, "needs_new"); !errors.Is(err, ErrLensReviewClosed) {
		t.Fatalf("review changed after confirmation: %v", err)
	}
	name, err := review.PrescriptionFile(ctx, orderID, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if path, err := files.Path(name); err != nil {
		t.Fatal(err)
	} else if data, err := os.ReadFile(path); err != nil || string(data) != "%PDF-1.7 qa" {
		t.Fatalf("stored prescription = %q %v", data, err)
	}
}
