package payments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

// PostgresStore persists payment methods and payments in PostgreSQL.
type PostgresStore struct{ DB *sql.DB }

// queryer is what both *sql.DB and *sql.Tx offer for reads.
type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// orderStateSQL reads what checkPayable and settles need; an open lens price is any line whose price is not final.
const orderStateSQL = `
	SELECT o.payment_status, o.order_status, o.total,
		EXISTS (SELECT 1 FROM order_items i WHERE i.order_id = o.id AND NOT i.lens_price_final),
		o.advance_required, o.amount_paid
	FROM orders o WHERE o.id = $1`

// readOrderState reads an order's payable state, locking the order row when lock is true.
func readOrderState(ctx context.Context, q queryer, orderID int, lock bool) (OrderState, error) {
	query := orderStateSQL
	if lock {
		query += " FOR UPDATE OF o"
	}
	var state OrderState
	err := q.QueryRowContext(ctx, query, orderID).Scan(&state.PaymentStatus, &state.OrderStatus, &state.Total, &state.PriceOpen, &state.Required, &state.Paid)
	if errors.Is(err, sql.ErrNoRows) {
		return OrderState{}, ErrNotFound
	}
	return state, err
}

// inTx runs work in one transaction and rolls back on any error.
func (s PostgresStore) inTx(ctx context.Context, work func(tx *sql.Tx) error) (resultErr error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback payment: %w", err))
		}
	}()
	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Method reads one method's settings.
func (s PostgresStore) Method(ctx context.Context, code string) (Method, error) {
	method := Method{Code: code}
	err := s.DB.QueryRowContext(ctx, `SELECT enabled, account_number, instructions FROM payment_methods WHERE code = $1`, code).
		Scan(&method.Enabled, &method.AccountNumber, &method.Instructions)
	if errors.Is(err, sql.ErrNoRows) {
		return Method{}, ErrNotFound
	}
	return method, err
}

// SaveMethod updates one existing method.
func (s PostgresStore) SaveMethod(ctx context.Context, method Method) error {
	result, err := s.DB.ExecContext(ctx, `
		UPDATE payment_methods SET enabled = $2, account_number = $3, instructions = $4, updated_at = NOW() WHERE code = $1`,
		method.Code, method.Enabled, method.AccountNumber, method.Instructions)
	if err != nil {
		return err
	}
	if changed, err := result.RowsAffected(); err != nil {
		return err
	} else if changed != 1 {
		return ErrNotFound
	}
	return nil
}

// OrderState reads the order's payable state without locking, for the customer's view.
func (s PostgresStore) OrderState(ctx context.Context, orderID int) (OrderState, error) {
	return readOrderState(ctx, s.DB, orderID, false)
}

// Submit saves a pending payment while the order row is locked, so it cannot race an approval or a cancellation.
// The partial unique index turns a repeated open submission into a read of the existing payment.
func (s PostgresStore) Submit(ctx context.Context, orderID int, sub Submission) (Payment, error) {
	var payment Payment
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		state, err := readOrderState(ctx, tx, orderID, true)
		if err != nil {
			return err
		}
		if err := checkPayable(state); err != nil {
			return err
		}
		// More than is still owed is a typo or a mistake; the customer would need a refund.
		if sub.Amount > state.Total-state.Paid {
			return ErrInvalidPayment
		}
		err = tx.QueryRowContext(ctx, `
			INSERT INTO payments (order_id, method, trx_id, sender_number, amount)
			SELECT $1, $2, $3, $4, $5
			WHERE (SELECT COUNT(*) FROM payments WHERE order_id = $1) < $6
			ON CONFLICT (order_id, method, trx_id) WHERE status <> 'rejected' DO NOTHING
			RETURNING id`, orderID, sub.Method, sub.TrxID, sub.SenderNumber, sub.Amount, maxPaymentsPerOrder).Scan(&payment.ID)
		if errors.Is(err, sql.ErrNoRows) {
			// Either the same open transaction exists already (idempotent repeat) or the order hit its limit.
			err = tx.QueryRowContext(ctx, `
				SELECT id FROM payments WHERE order_id = $1 AND method = $2 AND trx_id = $3 AND status <> 'rejected'`,
				orderID, sub.Method, sub.TrxID).Scan(&payment.ID)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrTooManyPayments
			}
		}
		if err != nil {
			return err
		}
		payment, err = scanPayment(tx.QueryRowContext(ctx, paymentSelectSQL+` WHERE p.id = $1`, payment.ID))
		return err
	})
	return payment, err
}

// paymentSelectSQL reads payments with their order number and the other orders that reuse the same transaction ID.
const paymentSelectSQL = `
	SELECT p.id, p.order_id, o.order_number, p.method, p.trx_id, p.sender_number, p.amount, p.status,
		p.reject_reason, p.created_at, p.reviewed_at,
		ARRAY(SELECT DISTINCT o2.order_number FROM payments p2 JOIN orders o2 ON o2.id = p2.order_id
			WHERE p2.method = p.method AND p2.trx_id = p.trx_id AND p2.order_id <> p.order_id ORDER BY o2.order_number)
	FROM payments p JOIN orders o ON o.id = p.order_id`

// rowScanner is a single row or the current row of a result set.
type rowScanner interface{ Scan(dest ...any) error }

// scanPayment reads one row of paymentSelectSQL.
func scanPayment(row rowScanner) (Payment, error) {
	var p Payment
	var reused pq.StringArray
	err := row.Scan(&p.ID, &p.OrderID, &p.OrderNumber, &p.Method, &p.TrxID, &p.SenderNumber, &p.Amount, &p.Status,
		&p.RejectReason, &p.CreatedAt, &p.ReviewedAt, &reused)
	if errors.Is(err, sql.ErrNoRows) {
		return Payment{}, ErrNotFound
	}
	p.UsedOnOrders = reused
	return p, err
}

// listPayments runs a payment query and reads every row.
func (s PostgresStore) listPayments(ctx context.Context, query string, args ...any) ([]Payment, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]Payment, 0)
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// ForOrder lists one order's payments, oldest first.
func (s PostgresStore) ForOrder(ctx context.Context, orderID int) ([]Payment, error) {
	return s.listPayments(ctx, paymentSelectSQL+` WHERE p.order_id = $1 ORDER BY p.id`, orderID)
}

// List returns the newest 200 payments, optionally of one status.
func (s PostgresStore) List(ctx context.Context, status string) ([]Payment, error) {
	return s.listPayments(ctx, paymentSelectSQL+` WHERE ($1 = '' OR p.status = $1) ORDER BY p.id DESC LIMIT 200`, status)
}

// review locks the payment and its order, checks the payment is still pending, then runs decide.
// Locking the order first matches Submit's order, so the two cannot deadlock.
func (s PostgresStore) review(ctx context.Context, paymentID int, decide func(tx *sql.Tx, orderID int, state OrderState) error) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var orderID int
		if err := tx.QueryRowContext(ctx, `SELECT order_id FROM payments WHERE id = $1`, paymentID).Scan(&orderID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		state, err := readOrderState(ctx, tx, orderID, true)
		if err != nil {
			return err
		}
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM payments WHERE id = $1 FOR UPDATE`, paymentID).Scan(&status); err != nil {
			return err
		}
		if status != StatusPending {
			return ErrAlreadyReviewed
		}
		return decide(tx, orderID, state)
	})
}

// Approve marks the payment approved, adds it to the order's paid amount and, once settles says the advance
// is covered, marks the order's advance received; all writes happen in one transaction.
func (s PostgresStore) Approve(ctx context.Context, paymentID int, adminID int64) error {
	return s.review(ctx, paymentID, func(tx *sql.Tx, orderID int, state OrderState) error {
		var amount int
		if err := tx.QueryRowContext(ctx, `SELECT amount FROM payments WHERE id = $1`, paymentID).Scan(&amount); err != nil {
			return err
		}
		if err := checkApprovable(state, amount); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE payments SET status = 'approved', reviewed_by = $2, reviewed_at = NOW() WHERE id = $1`, paymentID, adminID); err != nil {
			return err
		}
		// Only an order waiting for its advance changes status; a cash-on-delivery balance paid early just adds up.
		status := state.PaymentStatus
		if status == orderAdvanceDue && settles(state, amount) {
			status = orderAdvanceDone
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE orders SET amount_paid = amount_paid + $2, payment_status = $3, updated_at = NOW() WHERE id = $1`, orderID, amount, status)
		return err
	})
}

// Reject closes a pending payment with the owner's reason; the order is unchanged so the customer can try again.
func (s PostgresStore) Reject(ctx context.Context, paymentID int, adminID int64, reason string) error {
	return s.review(ctx, paymentID, func(tx *sql.Tx, _ int, _ OrderState) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE payments SET status = 'rejected', reject_reason = $3, reviewed_by = $2, reviewed_at = NOW() WHERE id = $1`,
			paymentID, adminID, reason)
		return err
	})
}

// Policies reads both advance rules, frame first.
func (s PostgresStore) Policies(ctx context.Context) ([]Policy, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT kind, mode, value FROM advance_policies ORDER BY kind`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Policy
	for rows.Next() {
		var p Policy
		if err := rows.Scan(&p.Kind, &p.Mode, &p.Value); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// Policy reads one advance rule.
func (s PostgresStore) Policy(ctx context.Context, kind string) (Policy, error) {
	p := Policy{Kind: kind}
	err := s.DB.QueryRowContext(ctx, `SELECT mode, value FROM advance_policies WHERE kind = $1`, kind).Scan(&p.Mode, &p.Value)
	if errors.Is(err, sql.ErrNoRows) {
		return Policy{}, ErrNotFound
	}
	return p, err
}

// SavePolicy updates one existing rule; it never touches orders, which keep their own copy.
func (s PostgresStore) SavePolicy(ctx context.Context, p Policy) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE advance_policies SET mode = $2, value = $3, updated_at = NOW() WHERE kind = $1`, p.Kind, p.Mode, p.Value)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.Join(err, ErrNotFound)
	}
	return nil
}
