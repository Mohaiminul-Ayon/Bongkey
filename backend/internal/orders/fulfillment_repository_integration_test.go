package orders

import (
	"context"
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
}
