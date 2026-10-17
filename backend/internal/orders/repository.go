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

// Find loads one order; missing rows become the same error as denied guest access.
func (r PostgresReader) Find(ctx context.Context, id string) (Record, error) {
	var record Record
	err := r.DB.QueryRowContext(ctx, `
		SELECT o.id, o.order_number, o.customer_name, o.customer_phone, o.shipping_address,
			o.city, o.subtotal, o.delivery_fee, o.total, o.order_status,
			COALESCE(i.invoice_number, ''), o.guest_token_hash
		FROM orders o LEFT JOIN invoices i ON i.order_id = o.id
		WHERE o.id::text = $1 OR o.order_number = $1
	`, id).Scan(
		&record.ID, &record.OrderNumber, &record.CustomerName, &record.CustomerPhone,
		&record.ShippingAddress, &record.City, &record.Subtotal, &record.DeliveryFee,
		&record.Total, &record.Status, &record.InvoiceNumber, &record.GuestTokenHash,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	return record, nil
}
