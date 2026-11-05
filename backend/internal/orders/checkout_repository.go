package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// PostgresCheckoutStore persists a complete checkout in PostgreSQL.
type PostgresCheckoutStore struct {
	DB *sql.DB
}

type postgresCheckoutTx struct {
	Tx *sql.Tx
}

// InTransaction commits only after every reservation, order item, and invoice succeeds.
func (s PostgresCheckoutStore) InTransaction(ctx context.Context, work func(CheckoutTx) error) (resultErr error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = fmt.Errorf("rollback checkout: %w", err)
		}
	}()
	if err := work(postgresCheckoutTx{Tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

// FindIdempotentOrder serializes one retry key across server instances and returns its committed receipt.
func (p postgresCheckoutTx) FindIdempotentOrder(ctx context.Context, keyHash string) (CheckoutReceipt, string, bool, error) {
	if _, err := p.Tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, keyHash); err != nil {
		return CheckoutReceipt{}, "", false, err
	}
	var receipt CheckoutReceipt
	var requestHash string
	err := p.Tx.QueryRowContext(ctx, `
		SELECT o.id, o.order_number, i.invoice_number, o.subtotal, o.delivery_fee, o.total, o.payment_method, o.idempotency_request_hash
		FROM orders o
		JOIN invoices i ON i.order_id = o.id
		WHERE o.idempotency_key_hash = $1
	`, keyHash).Scan(&receipt.ID, &receipt.OrderNumber, &receipt.InvoiceNumber, &receipt.Subtotal, &receipt.DeliveryFee, &receipt.Total, &receipt.PaymentMethod, &requestHash)
	if errors.Is(err, sql.ErrNoRows) {
		return CheckoutReceipt{}, "", false, nil
	}
	if err != nil {
		return CheckoutReceipt{}, "", false, err
	}
	return receipt, requestHash, true, nil
}

// ReserveProduct atomically rejects an inactive or insufficiently stocked product.
func (p postgresCheckoutTx) ReserveProduct(ctx context.Context, id, quantity int) (ReservedProduct, error) {
	var product ReservedProduct
	err := p.Tx.QueryRowContext(ctx, `
		UPDATE products AS p
		SET stock_quantity = stock_quantity - $2, updated_at = NOW()
		WHERE p.id = $1 AND p.is_active = true AND p.stock_quantity >= $2
			AND (p.parent_product_id IS NULL OR EXISTS (SELECT 1 FROM products root WHERE root.id = p.parent_product_id AND root.is_active = true))
		RETURNING p.name, p.price, p.sku
	`, id, quantity).Scan(&product.Name, &product.Price, &product.SKU)
	if errors.Is(err, sql.ErrNoRows) {
		return ReservedProduct{}, ErrUnavailable
	}
	return product, err
}

// LensForProduct reads an enabled lens type only when it is linked to the frame's category.
func (p postgresCheckoutTx) LensForProduct(ctx context.Context, packageID, productID int) (LensChoice, error) {
	var choice LensChoice
	err := p.Tx.QueryRowContext(ctx, `
		SELECT l.name, l.price, l.max_power, l.prescription_required
		FROM lens_packages l
		JOIN lens_package_categories c ON c.package_id = l.id
		JOIN products p ON p.category_id = c.category_id
		WHERE l.id = $1 AND p.id = $2 AND l.active
	`, packageID, productID).Scan(&choice.Name, &choice.Price, &choice.MaxPower, &choice.PrescriptionRequired)
	if errors.Is(err, sql.ErrNoRows) {
		return LensChoice{}, ErrLensUnavailable
	}
	return choice, err
}

// InsertOrder allocates its final unique number before inserting the order row.
func (p postgresCheckoutTx) InsertOrder(ctx context.Context, request CheckoutRequest, subtotal, deliveryFee, total int, tokenHash, idempotencyKeyHash, requestHash, paymentMethod, paymentStatus string) (int, string, error) {
	var id int
	if err := p.Tx.QueryRowContext(ctx, `SELECT nextval(pg_get_serial_sequence('orders', 'id'))`).Scan(&id); err != nil {
		return 0, "", err
	}
	number := fmt.Sprintf("ORD-%06d", id)
	_, err := p.Tx.ExecContext(ctx, `
		INSERT INTO orders (id, order_number, customer_name, customer_phone, customer_email, email_verified, shipping_address, city, delivery_note, subtotal, delivery_fee, total, guest_token_hash, idempotency_key_hash, idempotency_request_hash, payment_method, payment_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, id, number, request.CustomerName, request.CustomerPhone, request.CustomerEmail, request.EmailVerified, request.ShippingAddress, request.City, request.DeliveryNote, subtotal, deliveryFee, total, tokenHash, idempotencyKeyHash, requestHash, paymentMethod, paymentStatus)
	return id, number, err
}

// InsertItem snapshots the accepted product, lens choice, prices and quantity.
func (p postgresCheckoutTx) InsertItem(ctx context.Context, orderID int, item CheckoutItem, product ReservedProduct, line OrderLine) error {
	kind := item.LensKind
	if kind == "" {
		kind = "none"
	}
	var packageID *int
	if item.LensPackageID > 0 {
		packageID = &item.LensPackageID
	}
	_, err := p.Tx.ExecContext(ctx, `
		INSERT INTO order_items (order_id, product_id, product_name, sku, lens_option, quantity, unit_price, total_price,
			lens_kind, lens_package_id, lens_name, lens_power, sph_right, sph_left, lens_price, lens_price_final,
			prescription_file, prescription_status, lens_request)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`, orderID, item.ProductID, product.Name, product.SKU, item.LensOption, item.Quantity, product.Price, line.LineTotal,
		kind, packageID, line.LensName, item.LensPower, item.SPHRight, item.SPHLeft, line.LensPrice, line.LensPriceFinal,
		line.PrescriptionFile, line.PrescriptionStatus, line.LensRequest)
	return err
}

// InsertInvoice allocates a unique invoice number inside the checkout transaction.
func (p postgresCheckoutTx) InsertInvoice(ctx context.Context, orderID int, request CheckoutRequest, subtotal, deliveryFee, total int) (string, error) {
	var id int
	if err := p.Tx.QueryRowContext(ctx, `SELECT nextval(pg_get_serial_sequence('invoices', 'id'))`).Scan(&id); err != nil {
		return "", err
	}
	number := fmt.Sprintf("INV-%06d", id)
	_, err := p.Tx.ExecContext(ctx, `
		INSERT INTO invoices (id, invoice_number, order_id, customer_name, customer_phone, customer_email, billing_address, subtotal, delivery_fee, total)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, id, number, orderID, request.CustomerName, request.CustomerPhone, request.CustomerEmail, request.ShippingAddress, subtotal, deliveryFee, total)
	return number, err
}
