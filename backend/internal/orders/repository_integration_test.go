package orders

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"bongkey/backend/migrations"
	_ "github.com/lib/pq"
)

// TestPostgresOrderAccess exercises real SQL with session-local tables in an explicitly supplied test database.
func TestPostgresOrderAccess(t *testing.T) {
	dsn := os.Getenv("BONGKEY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BONGKEY_TEST_DATABASE_URL to an isolated test database to verify SQL and rollback")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal("could not open test database")
	}
	defer db.Close()
	// One retained connection keeps all temporary tables in the same session.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `
		CREATE TEMP TABLE products (id INTEGER PRIMARY KEY, name TEXT, price INTEGER, is_active BOOLEAN, stock_quantity INTEGER);
		CREATE TEMP TABLE stock_items (product_id INTEGER PRIMARY KEY, quantity INTEGER);
		CREATE TEMP TABLE orders (id SERIAL PRIMARY KEY, order_number TEXT UNIQUE, customer_name TEXT, customer_phone TEXT,
		customer_email TEXT, shipping_address TEXT, city TEXT, delivery_note TEXT, prescription_file_url TEXT,
		subtotal INTEGER, delivery_fee INTEGER, total INTEGER, payment_method TEXT DEFAULT 'cash_on_delivery', order_status TEXT DEFAULT 'pending');
		CREATE TEMP TABLE order_items (order_id INTEGER REFERENCES orders(id), product_id INTEGER, product_name TEXT,
		lens_option TEXT, quantity INTEGER, unit_price INTEGER, total_price INTEGER);
		CREATE TEMP TABLE invoices (id SERIAL PRIMARY KEY, invoice_number TEXT UNIQUE, order_id INTEGER UNIQUE REFERENCES orders(id),
		customer_name TEXT, customer_phone TEXT, customer_email TEXT, billing_address TEXT, subtotal INTEGER, delivery_fee INTEGER, total INTEGER);
		INSERT INTO products VALUES (1, 'Test frame', 1200, true, 10);
		INSERT INTO stock_items VALUES (1, 10);`)
	if err != nil {
		t.Fatal("could not create temporary test fixtures")
	}
	// Use the real migration's constraints without persisting its table in the test database.
	ddl := strings.Replace(migrations.OrderAccess, "CREATE TABLE IF NOT EXISTS", "CREATE TEMP TABLE", 1)
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		t.Fatal("access migration failed against temporary fixtures")
	}
	service := NewService(db)
	input := Input{CustomerName: "Test Customer", CustomerPhone: "01000000000", ShippingAddress: "Test address", City: "Dhaka", Items: []ItemInput{{ProductID: 1, Quantity: 1}}}
	first, err := service.Create(ctx, input)
	if err != nil {
		t.Fatal("first checkout failed")
	}
	second, err := service.Create(ctx, input)
	if err != nil {
		t.Fatal("second checkout failed")
	}
	if order, err := service.GetGuest(ctx, first.OrderNumber, first.AccessToken); err != nil || order.ID != first.ID {
		t.Fatal("owner access failed")
	}
	if _, err := service.GetGuest(ctx, second.OrderNumber, first.AccessToken); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-order access was not denied")
	}
	if _, err := service.GetGuest(ctx, "ORD-MISSING", first.AccessToken); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing order has a distinguishable error")
	}
	if _, err := db.ExecContext(ctx, `UPDATE order_access SET expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE order_id=$1`, first.ID); err != nil {
		t.Fatal("could not expire fixture")
	}
	if _, err := service.GetGuest(ctx, first.OrderNumber, first.AccessToken); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired access was not denied")
	}
	var hash string
	if err := db.QueryRowContext(ctx, `SELECT token_hash FROM order_access WHERE order_id=$1`, second.ID).Scan(&hash); err != nil || hash == second.AccessToken || len(hash) != 64 {
		t.Fatal("raw credential stored")
	}
	// Force the last checkout write to fail and verify all preceding writes roll back.
	if _, err := db.ExecContext(ctx, `ALTER TABLE order_access ADD CONSTRAINT reject_test_grant CHECK (order_id < 0) NOT VALID`); err != nil {
		t.Fatal("could not install failure fixture")
	}
	if result, err := service.Create(ctx, input); err == nil || result.AccessToken != "" {
		t.Fatal("failed grant returned a credential")
	}
	var count, invoices, items, stock, stockItems int
	err = db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM orders), (SELECT COUNT(*) FROM invoices),
		(SELECT COUNT(*) FROM order_items), (SELECT stock_quantity FROM products WHERE id=1), (SELECT quantity FROM stock_items WHERE product_id=1)`).
		Scan(&count, &invoices, &items, &stock, &stockItems)
	if err != nil || count != 2 || invoices != 2 || items != 2 || stock != 8 || stockItems != 8 {
		t.Fatal("checkout was not fully rolled back")
	}
}
