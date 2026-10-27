package orders

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"bongkey/backend/internal/security"
)

// TestVariantStockCheckoutAndCancelPostgres proves checkout and cancel touch only the ordered variant's row.
func TestVariantStockCheckoutAndCancelPostgres(t *testing.T) {
	db := localCheckoutDB(t)
	rootID := checkoutFixtureProduct(t, db, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var variantID int
	if err := db.QueryRowContext(ctx, `
		INSERT INTO products (parent_product_id, sku, name, slug, price, color, size, stock_quantity, is_active)
		VALUES ($1, $2, 'QA stock fixture', $2, 150, 'QA blue', 'QA wide', 2, true)
		RETURNING id
	`, rootID, fmt.Sprintf("qa-variant-%d-%d", os.Getpid(), time.Now().UnixNano())).Scan(&variantID); err != nil {
		t.Fatal("could not create local variant fixture")
	}
	// Registered after the root fixture, so it runs first and the root's RESTRICT parent key is free.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM products WHERE id = $1`, variantID); err != nil {
			t.Error("could not remove local variant fixture")
		}
	})
	proof, err := security.IssueEmailProof(testEmailProofSecret, "qa-buyer@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	service := CheckoutService{Store: PostgresCheckoutStore{DB: db}, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}
	fee := 80
	request := func(key string, quantity int) CheckoutRequest {
		return CheckoutRequest{
			IdempotencyKey: key, CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
			CustomerEmail: "qa-buyer@example.invalid", EmailProof: proof, ExpectedDeliveryFee: &fee,
			Items: []CheckoutItem{{ProductID: variantID, Quantity: quantity}},
		}
	}
	keyPrefix := fmt.Sprintf("variant-stock-%d-%d", os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM orders WHERE idempotency_key_hash IN ($1, $2) AND customer_name = 'QA'`, textHash(keyPrefix+"-over"), textHash(keyPrefix+"-ok")); err != nil {
			t.Error("could not remove variant checkout fixture")
		}
	})
	stocks := func() (int, int) {
		t.Helper()
		var root, variant int
		if err := db.QueryRowContext(ctx, `SELECT (SELECT stock_quantity FROM products WHERE id = $1), (SELECT stock_quantity FROM products WHERE id = $2)`, rootID, variantID).Scan(&root, &variant); err != nil {
			t.Fatal("could not read variant stock")
		}
		return root, variant
	}

	if _, err := service.Place(ctx, request(keyPrefix+"-over", 3)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ordering more than variant stock: err=%v, want ErrUnavailable", err)
	}
	if root, variant := stocks(); root != 5 || variant != 2 {
		t.Fatalf("after rejected order root=%d variant=%d, want 5 and 2", root, variant)
	}

	receipt, err := service.Place(ctx, request(keyPrefix+"-ok", 2))
	if err != nil {
		t.Fatalf("variant checkout failed: %v", err)
	}
	if receipt.Subtotal != 300 {
		t.Fatalf("subtotal=%d, want the variant's own price 2x150", receipt.Subtotal)
	}
	if root, variant := stocks(); root != 5 || variant != 0 {
		t.Fatalf("after checkout root=%d variant=%d, want 5 and 0", root, variant)
	}

	if _, err := (FulfillmentService{Store: PostgresFulfillmentStore{DB: db}}).ChangeStatus(ctx, receipt.ID, "cancelled"); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if root, variant := stocks(); root != 5 || variant != 2 {
		t.Fatalf("after cancel root=%d variant=%d, want 5 and 2", root, variant)
	}
}
