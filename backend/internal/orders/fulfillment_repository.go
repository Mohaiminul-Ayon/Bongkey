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

// UpdateStatus rejects an unexpected concurrent state instead of overwriting it.
func (p postgresFulfillmentTx) UpdateStatus(ctx context.Context, orderID int, current, next string) error {
	result, err := p.Tx.ExecContext(ctx, `UPDATE orders SET order_status = $3, updated_at = NOW() WHERE id = $1 AND order_status = $2`, orderID, current, next)
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
