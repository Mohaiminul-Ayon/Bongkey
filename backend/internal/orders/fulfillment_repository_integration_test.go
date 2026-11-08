package orders

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// TestConcurrentCancellationPostgres proves two cancellation retries restore stock only once.
func TestConcurrentCancellationPostgres(t *testing.T) {
	db := localCheckoutDB(t)
	productID := checkoutFixtureProduct(t, db, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	orderNumber := fmt.Sprintf("QA-FULFILL-%d-%d", os.Getpid(), time.Now().UnixNano())
	var orderID int
	if err := db.QueryRowContext(ctx, `
		INSERT INTO orders (order_number, customer_name, customer_phone, shipping_address, city, subtotal, delivery_fee, total)
		VALUES ($1, 'QA fulfillment', '000', 'Local', 'Dhaka', 200, 80, 280)
		RETURNING id
	`, orderNumber).Scan(&orderID); err != nil {
		t.Fatal("could not create fulfillment order fixture")
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO order_items (order_id, product_id, product_name, quantity, unit_price, total_price)
		VALUES ($1, $2, 'QA stock fixture', 2, 100, 200)
	`, orderID, productID); err != nil {
		t.Fatal("could not create fulfillment item fixture")
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM orders WHERE id = $1 AND order_number = $2`, orderID, orderNumber); err != nil {
			t.Error("could not remove fulfillment order fixture")
		}
	})

	service := FulfillmentService{Store: PostgresFulfillmentStore{DB: db}}
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for i := 0; i < 2; i++ {
		// The test owns both workers; the shared timeout bounds row-lock waits and Wait observes completion.
		go func() {
			defer workers.Done()
			<-start
			_, err := service.ChangeStatus(ctx, orderID, "cancelled")
			results <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent cancellation failed: %v", err)
		}
	}

	var status string
	var stock int
	if err := db.QueryRowContext(ctx, `SELECT order_status FROM orders WHERE id = $1`, orderID).Scan(&status); err != nil {
		t.Fatal("could not read cancelled order")
	}
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, productID).Scan(&stock); err != nil {
		t.Fatal("could not read restored stock")
	}
	if status != "cancelled" || stock != 3 {
		t.Fatalf("status=%q stock=%d, want cancelled and 3", status, stock)
	}
	// Two racing cancellations restore once, so exactly one restore row, tied to the order.
	var restoreRows, restoreOrder int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*), MAX(order_id) FROM stock_history WHERE product_id = $1 AND reason = 'order_cancelled' AND change = 2`, productID).Scan(&restoreRows, &restoreOrder); err != nil || restoreRows != 1 || restoreOrder != orderID {
		t.Fatalf("restore rows=%d order=%d err=%v", restoreRows, restoreOrder, err)
	}
}

// TestDeliveryTrackingPostgres walks one order to delivery: each step gets its time once, courier details need a
// confirmed order, delivered can never be stored before shipped, and the public lookup needs the right phone.
func TestDeliveryTrackingPostgres(t *testing.T) {
	db := localCheckoutDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var orderID int
	var orderNumber string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO orders (order_number, customer_name, customer_phone, shipping_address, city, subtotal, delivery_fee, total)
		VALUES ('QA-TRACK-' || $1, 'Rahim QA', '01712345678', 'House 5', 'Dhaka', 200, 80, 280)
		RETURNING id`, fmt.Sprint(time.Now().UnixNano())).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	// The public lookup only accepts ORD- numbers, so give the fixture one that cannot clash with real orders.
	orderNumber = fmt.Sprintf("ORD-%d", 900000000+orderID)
	if _, err := db.ExecContext(ctx, `UPDATE orders SET order_number = $2 WHERE id = $1`, orderID, orderNumber); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO order_items (order_id, product_name, quantity, unit_price, total_price) VALUES ($1, 'QA frame', 1, 200, 200)`, orderID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM orders WHERE id = $1`, orderID) })

	tracking := TrackingService{Store: PostgresFulfillmentStore{DB: db}}
	if err := tracking.SetTracking(ctx, orderID, "Steadfast", "SF-1", ""); !errors.Is(err, ErrTrackingClosed) {
		t.Fatalf("tracking on a pending order: %v", err)
	}
	fulfillment := FulfillmentService{Store: PostgresFulfillmentStore{DB: db}}
	for _, step := range []string{"confirmed", "preparing", "shipped"} {
		if _, err := fulfillment.ChangeStatus(ctx, orderID, step); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}
	if err := tracking.SetTracking(ctx, orderID, "Steadfast", "SF-1", "https://steadfast.com.bd/t/SF-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE orders SET delivered_at = shipped_at - INTERVAL '1 hour' WHERE id = $1`, orderID); err == nil {
		t.Fatal("database accepted a delivered time before the ship time")
	}
	if _, err := fulfillment.ChangeStatus(ctx, orderID, "delivered"); err != nil {
		t.Fatal(err)
	}

	access := AccessService{Reader: PostgresReader{DB: db}, Limiter: countingLimiter{}, EmailProofSecret: testEmailProofSecret}
	if _, err := access.Track(ctx, orderNumber, "01799999999", "203.0.113.20"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong phone: %v", err)
	}
	view, err := access.Track(ctx, orderNumber, "+8801712345678", "203.0.113.20")
	if err != nil {
		t.Fatal(err)
	}
	tr := view.Tracking
	if view.Status != "delivered" || tr.ConfirmedAt == nil || tr.PreparingAt == nil || tr.ShippedAt == nil || tr.DeliveredAt == nil || tr.CancelledAt != nil {
		t.Fatalf("step times = %+v", tr)
	}
	if tr.DeliveredAt.Before(*tr.ShippedAt) || tr.Courier != "Steadfast" || tr.TrackingURL != "https://steadfast.com.bd/t/SF-1" || view.Customer != "Ra***" {
		t.Fatalf("tracking view = %+v", view)
	}
	if _, err := fulfillment.ChangeStatus(ctx, orderID, "delivered"); err != nil {
		t.Fatalf("repeated delivered request: %v", err)
	}
	again, _ := access.Track(ctx, orderNumber, "01712345678", "203.0.113.20")
	if !again.Tracking.DeliveredAt.Equal(*tr.DeliveredAt) {
		t.Fatal("delivered time was overwritten")
	}
}
