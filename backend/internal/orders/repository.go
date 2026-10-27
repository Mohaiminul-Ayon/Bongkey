package orders

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
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

// AdminOrderPageSize bounds one admin order list response so the page stays fast as orders grow.
const AdminOrderPageSize = 50

// ListAdmin returns one page of order summaries older than beforeID (0 means newest), without guest access hashes.
// Keyset paging on the primary key stays stable while new orders arrive, unlike OFFSET.
func (r PostgresReader) ListAdmin(ctx context.Context, beforeID int) ([]AdminOrder, error) {
	if r.DB == nil {
		return nil, errors.New("order storage unavailable")
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, order_number, customer_name, customer_phone, city, total, order_status, created_at
		FROM orders WHERE ($1 = 0 OR id < $1) ORDER BY id DESC LIMIT $2
	`, beforeID, AdminOrderPageSize)
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

// findOrderSQL is shared by both lookups; only the fixed WHERE clause differs, never user text.
const findOrderSQL = `
	SELECT o.id, o.order_number, o.customer_name, o.customer_phone, o.shipping_address,
		o.city, o.subtotal, o.delivery_fee, o.total, o.order_status,
		COALESCE(i.invoice_number, ''), o.guest_token_hash, o.customer_email, o.email_verified
	FROM orders o LEFT JOIN invoices i ON i.order_id = o.id
	WHERE `

// Find loads one order by numeric ID or order number; missing rows become the same error as denied guest access.
func (r PostgresReader) Find(ctx context.Context, id string) (Record, error) {
	// Comparing the typed key keeps the primary-key and unique order_number indexes usable; id::text forced a scan.
	query, arg := findOrderSQL+"o.order_number = $1", any(id)
	if numericID, err := strconv.Atoi(id); err == nil {
		if numericID <= 0 {
			return Record{}, ErrNotFound
		}
		query, arg = findOrderSQL+"o.id = $1", numericID
	}
	var record Record
	err := r.DB.QueryRowContext(ctx, query, arg).Scan(
		&record.ID, &record.OrderNumber, &record.CustomerName, &record.CustomerPhone,
		&record.ShippingAddress, &record.City, &record.Subtotal, &record.DeliveryFee,
		&record.Total, &record.Status, &record.InvoiceNumber, &record.GuestTokenHash,
		&record.CustomerEmail, &record.EmailVerified,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	record.Items, err = r.items(ctx, record.ID)
	if err != nil {
		return Record{}, err
	}
	return record, nil
}

// items loads the checkout snapshot lines so fulfillment never depends on current product rows.
func (r PostgresReader) items(ctx context.Context, orderID int) ([]Item, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT product_name, lens_option, quantity, unit_price, total_price
		FROM order_items WHERE order_id = $1 ORDER BY id
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Item, 0)
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ProductName, &item.LensOption, &item.Quantity, &item.UnitPrice, &item.TotalPrice); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
