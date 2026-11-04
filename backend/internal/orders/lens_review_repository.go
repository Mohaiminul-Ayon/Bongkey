package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// PostgresLensReviewStore persists owner lens decisions in PostgreSQL.
type PostgresLensReviewStore struct{ DB *sql.DB }

// inLockedOrder runs work while the order row is locked, after checking it is still pending.
func (s PostgresLensReviewStore) inLockedOrder(ctx context.Context, orderID int, work func(tx *sql.Tx, paymentStatus string) error) (resultErr error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback lens review: %w", err))
		}
	}()
	var status, paymentStatus string
	err = tx.QueryRowContext(ctx, `SELECT order_status, payment_status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&status, &paymentStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "pending" {
		return ErrLensReviewClosed
	}
	if err := work(tx, paymentStatus); err != nil {
		return err
	}
	return tx.Commit()
}

// SetPrescriptionStatus changes only lines that carry a prescription check.
func (s PostgresLensReviewStore) SetPrescriptionStatus(ctx context.Context, orderID, itemID int, status string) error {
	return s.inLockedOrder(ctx, orderID, func(tx *sql.Tx, _ string) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE order_items SET prescription_status = $3
			WHERE id = $2 AND order_id = $1 AND prescription_status <> ''`, orderID, itemID, status)
		return oneRow(result, err)
	})
}

// SetLensPrice settles one lens price and recomputes the order and invoice totals in the same transaction.
// Prices are locked once the advance is recorded, because the customer has paid against them.
func (s PostgresLensReviewStore) SetLensPrice(ctx context.Context, orderID, itemID, price int, customName string) error {
	return s.inLockedOrder(ctx, orderID, func(tx *sql.Tx, paymentStatus string) error {
		if paymentStatus != "advance_due" {
			return ErrLensReviewClosed
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE order_items SET
				lens_price = $3, lens_price_final = true, total_price = (unit_price + $3) * quantity,
				lens_name = CASE WHEN lens_kind = 'custom' AND $4 <> '' THEN $4 ELSE lens_name END,
				lens_option = CASE WHEN lens_kind = 'custom' AND $4 <> '' THEN 'Custom lens · ' || $4 ELSE lens_option END
			WHERE id = $2 AND order_id = $1 AND lens_kind <> 'none'`, orderID, itemID, price, customName)
		if err := oneRow(result, err); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE orders o SET subtotal = s.sum, total = s.sum + o.delivery_fee, updated_at = NOW()
			FROM (SELECT SUM(total_price)::INTEGER AS sum FROM order_items WHERE order_id = $1) s
			WHERE o.id = $1`, orderID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE invoices i SET subtotal = o.subtotal, total = o.total
			FROM orders o WHERE o.id = $1 AND i.order_id = o.id`, orderID)
		return err
	})
}

// MarkAdvanceReceived records the advance only after every lens price on the order is final.
func (s PostgresLensReviewStore) MarkAdvanceReceived(ctx context.Context, orderID int) error {
	return s.inLockedOrder(ctx, orderID, func(tx *sql.Tx, paymentStatus string) error {
		if paymentStatus != "advance_due" {
			return ErrLensReviewClosed
		}
		var open bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM order_items WHERE order_id = $1 AND NOT lens_price_final)`, orderID).Scan(&open); err != nil {
			return err
		}
		if open {
			return ErrLensPriceNotFinal
		}
		_, err := tx.ExecContext(ctx, `UPDATE orders SET payment_status = 'advance_received', updated_at = NOW() WHERE id = $1`, orderID)
		return err
	})
}

// PrescriptionFile reads the stored file name of one line of one order.
func (s PostgresLensReviewStore) PrescriptionFile(ctx context.Context, orderID, itemID int) (string, error) {
	var name string
	err := s.DB.QueryRowContext(ctx, `SELECT prescription_file FROM order_items WHERE id = $2 AND order_id = $1`, orderID, itemID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && name == "") {
		return "", ErrNotFound
	}
	return name, err
}

// oneRow turns "no matching line" into ErrNotFound.
func oneRow(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrNotFound
	}
	return nil
}
