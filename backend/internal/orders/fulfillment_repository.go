package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// PostgresFulfillmentStore persists fulfillment changes in PostgreSQL.
type PostgresFulfillmentStore struct {
	DB *sql.DB
}

type postgresFulfillmentTx struct {
	Tx *sql.Tx
}

// InTransaction commits the status and stock effects together or rolls both back.
func (s PostgresFulfillmentStore) InTransaction(ctx context.Context, work func(FulfillmentTx) error) (resultErr error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback fulfillment: %w", err))
		}
	}()
	if err := work(postgresFulfillmentTx{Tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

// LockOrder returns the current state while blocking competing fulfillment writes.
func (p postgresFulfillmentTx) LockOrder(ctx context.Context, orderID int) (string, error) {
	var status string
	err := p.Tx.QueryRowContext(ctx, `SELECT order_status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return status, err
}

// RestoreStock returns each surviving product's aggregated order quantity once inside the locked transaction.
func (p postgresFulfillmentTx) RestoreStock(ctx context.Context, orderID int) error {
	// Lock product rows in ascending ID order, the same order checkout reserves them, so a cancel
	// racing a checkout of the same products waits instead of deadlocking.
	if _, err := p.Tx.ExecContext(ctx, `
		SELECT id FROM products
		WHERE id IN (SELECT product_id FROM order_items WHERE order_id = $1 AND product_id IS NOT NULL)
		ORDER BY id FOR UPDATE
	`, orderID); err != nil {
		return err
	}
	// Labels the restock for the stock_history trigger; the settings end with this transaction.
	if _, err := p.Tx.ExecContext(ctx, `SELECT set_config('bongkey.stock_reason', 'order_cancelled', true), set_config('bongkey.stock_order', $1::TEXT, true)`, orderID); err != nil {
		return err
	}
	_, err := p.Tx.ExecContext(ctx, `
		WITH restored AS (
			SELECT product_id, SUM(quantity)::INTEGER AS quantity
			FROM order_items
			WHERE order_id = $1 AND product_id IS NOT NULL
			GROUP BY product_id
		)
		UPDATE products AS product
		SET stock_quantity = product.stock_quantity + restored.quantity, updated_at = NOW()
		FROM restored
		WHERE product.id = restored.product_id
	`, orderID)
	return err
}

// LensReadiness checks a lens order's open steps while the order row is locked.
func (p postgresFulfillmentTx) LensReadiness(ctx context.Context, orderID int) error {
	var method, payment string
	var openPrice, openPrescription bool
	err := p.Tx.QueryRowContext(ctx, `
		SELECT o.payment_method, o.payment_status,
			EXISTS (SELECT 1 FROM order_items WHERE order_id = o.id AND NOT lens_price_final),
			EXISTS (SELECT 1 FROM order_items WHERE order_id = o.id AND prescription_status IN ('pending', 'needs_new'))
		FROM orders o WHERE o.id = $1`, orderID).Scan(&method, &payment, &openPrice, &openPrescription)
	switch {
	case err != nil:
		return err
	case openPrice:
		return ErrLensPriceNotFinal
	case openPrescription:
		return ErrPrescriptionNotAccepted
	case payment == "advance_due":
		return ErrAdvanceNotReceived
	}
	return nil
}

// UpdateStatus rejects an unexpected concurrent state instead of overwriting it.
func (p postgresFulfillmentTx) UpdateStatus(ctx context.Context, orderID int, current, next string) error {
	// Each step's time is written once, by the change into that step; the state machine never re-enters a step,
	// so the delivered time that starts the return window cannot be silently overwritten.
	result, err := p.Tx.ExecContext(ctx, `
		UPDATE orders SET order_status = $3, updated_at = NOW(),
			confirmed_at = CASE WHEN $3 = 'confirmed' THEN COALESCE(confirmed_at, NOW()) ELSE confirmed_at END,
			preparing_at = CASE WHEN $3 = 'preparing' THEN COALESCE(preparing_at, NOW()) ELSE preparing_at END,
			shipped_at = CASE WHEN $3 = 'shipped' THEN COALESCE(shipped_at, NOW()) ELSE shipped_at END,
			delivered_at = CASE WHEN $3 = 'delivered' THEN COALESCE(delivered_at, NOW()) ELSE delivered_at END,
			cancelled_at = CASE WHEN $3 = 'cancelled' THEN COALESCE(cancelled_at, NOW()) ELSE cancelled_at END
		WHERE id = $1 AND order_status = $2`, orderID, current, next)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrFulfillmentConflict
	}
	return nil
}

// SaveTracking stores courier details in one statement that also checks the order is trackable,
// so a concurrent cancellation cannot slip between the check and the write.
func (s PostgresFulfillmentStore) SaveTracking(ctx context.Context, orderID int, courier, code, link string) error {
	result, err := s.DB.ExecContext(ctx, `
		UPDATE orders SET courier_name = $2, tracking_code = $3, tracking_url = $4, updated_at = NOW()
		WHERE id = $1 AND order_status IN ('confirmed', 'preparing', 'shipped', 'delivered')`, orderID, courier, code, link)
	if err != nil {
		return err
	}
	if changed, err := result.RowsAffected(); err != nil || changed == 1 {
		return err
	}
	var exists bool
	if err := s.DB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM orders WHERE id = $1)`, orderID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return ErrTrackingClosed
}
