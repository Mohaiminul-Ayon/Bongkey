package notifications

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// PostgresStore keeps the email queue in the notifications table.
type PostgresStore struct {
	DB *sql.DB
}

// staleClaim is how long a row may stay "sending" before another tick retries it, for example after a crash.
// shortcut: a crash between a provider accepting an email and MarkSent can send that one email twice; acceptable for shop notices.
const staleClaim = 10 * time.Minute

// Enqueue inserts one email; a repeated event key is ignored, which makes every event idempotent.
func (s PostgresStore) Enqueue(ctx context.Context, out Outgoing) error {
	status := "pending"
	if out.Skip {
		status = "skipped"
	}
	var invoiceOrder any
	if out.InvoiceOrderID > 0 {
		invoiceOrder = out.InvoiceOrderID
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO notifications (event_key, recipient, subject, body, invoice_order_id, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (event_key) DO NOTHING`, out.Key, out.To, out.Subject, out.Text, invoiceOrder, status)
	return err
}

// HasEvent reports whether an event, such as one day's summary, was already queued.
func (s PostgresStore) HasEvent(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := s.DB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM notifications WHERE event_key = $1)`, key).Scan(&exists)
	return exists, err
}

// Payment reads one submitted payment for its email.
func (s PostgresStore) Payment(ctx context.Context, paymentID int) (PaymentNotice, error) {
	var p PaymentNotice
	err := s.DB.QueryRowContext(ctx, `
		SELECT order_id, amount, status, reject_reason, trx_id, sender_number FROM payments WHERE id = $1`, paymentID).
		Scan(&p.OrderID, &p.Amount, &p.Status, &p.RejectReason, &p.TrxID, &p.SenderNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return PaymentNotice{}, errors.New("payment not found")
	}
	return p, err
}

// ClaimDue locks due rows with SKIP LOCKED, so two servers never claim the same email, and counts the attempt.
func (s PostgresStore) ClaimDue(ctx context.Context, limit int) ([]Queued, error) {
	rows, err := s.DB.QueryContext(ctx, `
		UPDATE notifications n SET status = 'sending', attempts = n.attempts + 1, claimed_at = NOW()
		WHERE n.id IN (
			SELECT id FROM notifications
			WHERE (status = 'pending' AND next_attempt_at <= NOW())
				OR (status = 'sending' AND claimed_at < NOW() - make_interval(secs => $2))
			ORDER BY id LIMIT $1
			FOR UPDATE SKIP LOCKED)
		RETURNING n.id, n.recipient, n.subject, n.body, COALESCE(n.invoice_order_id, 0), n.attempts`, limit, staleClaim.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var queued []Queued
	for rows.Next() {
		var q Queued
		if err := rows.Scan(&q.ID, &q.To, &q.Subject, &q.Text, &q.InvoiceOrderID, &q.Attempts); err != nil {
			return nil, err
		}
		queued = append(queued, q)
	}
	return queued, rows.Err()
}

// MarkSent records a delivered email.
func (s PostgresStore) MarkSent(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE notifications SET status = 'sent', sent_at = NOW(), last_error = '' WHERE id = $1`, id)
	return err
}

// MarkSkipped records an email that must not be sent, such as one to a reserved test address.
func (s PostgresStore) MarkSkipped(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE notifications SET status = 'skipped' WHERE id = $1`, id)
	return err
}

// MarkRetry puts an email back in the queue for a later attempt.
func (s PostgresStore) MarkRetry(ctx context.Context, id int64, next time.Time, reason string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE notifications SET status = 'pending', next_attempt_at = $2, last_error = $3 WHERE id = $1`, id, next, reason)
	return err
}

// MarkFailed ends retrying; the owner's summary counts recent failures.
func (s PostgresStore) MarkFailed(ctx context.Context, id int64, reason string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE notifications SET status = 'failed', last_error = $2 WHERE id = $1`, id, reason)
	return err
}

// summaryOrderLimit keeps the evening email short on a busy day; counts still cover every order.
const summaryOrderLimit = 20

// Summary counts today's orders and everything still waiting for the owner.
func (s PostgresStore) Summary(ctx context.Context, from, to time.Time) (DailySummary, error) {
	var d DailySummary
	err := s.DB.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM orders WHERE created_at >= $1 AND created_at < $2),
			(SELECT COALESCE(SUM(total), 0) FROM orders WHERE created_at >= $1 AND created_at < $2),
			(SELECT COUNT(*) FROM payments WHERE status = 'pending'),
			(SELECT COUNT(DISTINCT i.order_id) FROM order_items i JOIN orders o ON o.id = i.order_id
				WHERE i.prescription_status IN ('pending', 'needs_new') AND o.order_status NOT IN ('cancelled', 'delivered')),
			(SELECT COUNT(DISTINCT i.order_id) FROM order_items i JOIN orders o ON o.id = i.order_id
				WHERE NOT i.lens_price_final AND o.order_status <> 'cancelled'),
			(SELECT COUNT(*) FROM orders WHERE order_status IN ('confirmed', 'preparing')),
			(SELECT COUNT(*) FROM notifications WHERE status = 'failed' AND created_at >= $2::timestamptz - INTERVAL '1 day')`, from, to).
		Scan(&d.NewOrderCount, &d.NewOrdersTotal, &d.PaymentsWaiting, &d.Prescriptions, &d.LensPrices, &d.NotShipped, &d.FailedEmails)
	if err != nil {
		return DailySummary{}, err
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT order_number, total, order_status FROM orders
		WHERE created_at >= $1 AND created_at < $2 ORDER BY id LIMIT $3`, from, to, summaryOrderLimit)
	if err != nil {
		return DailySummary{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var o SummaryOrder
		if err := rows.Scan(&o.OrderNumber, &o.Total, &o.Status); err != nil {
			return DailySummary{}, err
		}
		d.NewOrders = append(d.NewOrders, o)
	}
	return d, rows.Err()
}
