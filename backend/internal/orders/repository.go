package orders

import (
	"context"
	"database/sql"
	"errors"
)

// PostgresReader reads order details and the guest access hash from PostgreSQL.
type PostgresReader struct {
	DB *sql.DB
}

// AdminOrder is the summary shown in the owner order list.
type AdminOrder struct {
	ID            int    `json:"id"`
	OrderNumber   string `json:"order_number"`
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	City          string `json:"city"`
	Total         int    `json:"total"`
	OrderStatus   string `json:"order_status"`
	CreatedAt     string `json:"created_at"`
}

// ListAdmin returns recent order summaries without exposing guest access hashes.
func (r PostgresReader) ListAdmin(ctx context.Context) ([]AdminOrder, error) {
	if r.DB == nil {
		return nil, errors.New("order storage unavailable")
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT id, order_number, customer_name, customer_phone, city, total, order_status, created_at FROM orders ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]AdminOrder, 0)
	for rows.Next() {
		var order AdminOrder
		if err := rows.Scan(&order.ID, &order.OrderNumber, &order.CustomerName, &order.CustomerPhone, &order.City, &order.Total, &order.OrderStatus, &order.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

// Find loads one order; missing rows become the same error as denied guest access.
func (r PostgresReader) Find(ctx context.Context, id string) (Record, error) {
	var record Record
	err := r.DB.QueryRowContext(ctx, `
		SELECT o.id, o.order_number, o.customer_name, o.customer_phone, o.shipping_address,
			o.city, o.subtotal, o.delivery_fee, o.total, o.order_status,
			COALESCE(i.invoice_number, ''), o.guest_token_hash, o.customer_email, o.email_verified,
			o.delivery_note, o.payment_method, o.payment_status, o.created_at,
			COALESCE(i.issued_at::text, '')
		FROM orders o LEFT JOIN invoices i ON i.order_id = o.id
		WHERE o.id::text = $1 OR o.order_number = $1
	`, id).Scan(
		&record.ID, &record.OrderNumber, &record.CustomerName, &record.CustomerPhone,
		&record.ShippingAddress, &record.City, &record.Subtotal, &record.DeliveryFee,
		&record.Total, &record.Status, &record.InvoiceNumber, &record.GuestTokenHash,
		&record.CustomerEmail, &record.EmailVerified, &record.DeliveryNote,
		&record.PaymentMethod, &record.PaymentStatus, &record.CreatedAt, &record.InvoiceIssuedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	return record, nil
}

// Items loads immutable line snapshots for a previously authorized admin order.
func (r PostgresReader) Items(ctx context.Context, orderID int) ([]Item, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT product_name, sku, lens_option, quantity, unit_price, total_price
		FROM order_items WHERE order_id = $1 ORDER BY id
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Item, 0)
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ProductName, &item.SKU, &item.LensOption, &item.Quantity, &item.UnitPrice, &item.TotalPrice); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
