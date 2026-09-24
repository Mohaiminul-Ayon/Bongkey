package orders

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type sqlRepository struct{ db *sql.DB }
type sqlCheckoutTx struct{ tx *sql.Tx }

// transact rolls back all checkout writes on failure, including access-grant failures.
func (r sqlRepository) transact(ctx context.Context, work func(checkoutTx) error) error {
	if r.db == nil {
		return fmt.Errorf("database unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(sqlCheckoutTx{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

// findGuest applies ownership and expiration in the query before returning private fields.
func (r sqlRepository) findGuest(ctx context.Context, reference, digest string) (Order, error) {
	if r.db == nil {
		return Order{}, fmt.Errorf("database unavailable")
	}
	var order Order
	err := r.db.QueryRowContext(ctx, `
		SELECT o.id, o.order_number, o.customer_name, o.customer_phone, o.shipping_address,
		       o.city, o.subtotal, o.delivery_fee, o.total, o.payment_method, o.order_status,
		       COALESCE(i.invoice_number, '')
		FROM orders o JOIN order_access a ON a.order_id = o.id
		LEFT JOIN invoices i ON i.order_id = o.id
		WHERE (o.id::text = $1 OR o.order_number = $1)
		  AND a.token_hash = $2 AND a.expires_at > CURRENT_TIMESTAMP`, reference, digest).
		Scan(&order.ID, &order.OrderNumber, &order.CustomerName, &order.CustomerPhone,
			&order.ShippingAddress, &order.City, &order.Subtotal, &order.DeliveryFee,
			&order.Total, &order.PaymentMethod, &order.OrderStatus, &order.InvoiceNumber)
	return order, err
}

// product reads the authoritative price for an active catalog item.
func (t sqlCheckoutTx) product(ctx context.Context, id int) (string, int, error) {
	var name string
	var price int
	err := t.tx.QueryRowContext(ctx, `SELECT name, price FROM products WHERE id=$1 AND is_active=true`, id).Scan(&name, &price)
	return name, price, err
}

// createOrder persists the accepted customer details and totals inside checkout.
func (t sqlCheckoutTx) createOrder(ctx context.Context, input Input, subtotal, delivery int) (int, string, error) {
	var id int
	err := t.tx.QueryRowContext(ctx, `INSERT INTO orders
		(order_number, customer_name, customer_phone, customer_email, shipping_address, city,
		 delivery_note, prescription_file_url, subtotal, delivery_fee, total)
		VALUES ('TEMP', $1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		input.CustomerName, input.CustomerPhone, input.CustomerEmail, input.ShippingAddress,
		input.City, input.DeliveryNote, input.PrescriptionFileURL, subtotal, delivery, subtotal+delivery).Scan(&id)
	if err != nil {
		return 0, "", err
	}
	number := fmt.Sprintf("ORD-%06d", id)
	_, err = t.tx.ExecContext(ctx, `UPDATE orders SET order_number=$1 WHERE id=$2`, number, id)
	return id, number, err
}

// addItem records a line and applies inherited stock updates; oversell prevention is FIX-04.
func (t sqlCheckoutTx) addItem(ctx context.Context, id int, item pricedItem) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO order_items
		(order_id, product_id, product_name, lens_option, quantity, unit_price, total_price)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, id, item.ProductID, item.Name, item.LensOption, item.Quantity, item.Price, item.Price*item.Quantity)
	if err != nil {
		return err
	}
	if _, err = t.tx.ExecContext(ctx, `UPDATE products SET stock_quantity=GREATEST(stock_quantity-$1,0) WHERE id=$2`, item.Quantity, item.ProductID); err != nil {
		return err
	}
	_, err = t.tx.ExecContext(ctx, `UPDATE stock_items SET quantity=GREATEST(quantity-$1,0) WHERE product_id=$2`, item.Quantity, item.ProductID)
	return err
}

// createInvoice records an invoice summary without claiming a PDF has been generated.
func (t sqlCheckoutTx) createInvoice(ctx context.Context, id int, input Input, subtotal, delivery int) (string, error) {
	var invoiceID int
	err := t.tx.QueryRowContext(ctx, `INSERT INTO invoices
		(invoice_number, order_id, customer_name, customer_phone, customer_email, billing_address, subtotal, delivery_fee, total)
		VALUES ('TEMP',$1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, id, input.CustomerName,
		input.CustomerPhone, input.CustomerEmail, input.ShippingAddress, subtotal, delivery, subtotal+delivery).Scan(&invoiceID)
	if err != nil {
		return "", err
	}
	number := fmt.Sprintf("INV-%06d", invoiceID)
	_, err = t.tx.ExecContext(ctx, `UPDATE invoices SET invoice_number=$1 WHERE id=$2`, number, invoiceID)
	return number, err
}

// saveAccess persists only the digest, in the same transaction as its order.
func (t sqlCheckoutTx) saveAccess(ctx context.Context, id int, digest string, expires time.Time) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO order_access (order_id, token_hash, expires_at) VALUES ($1,$2,$3)`, id, digest, expires)
	return err
}
