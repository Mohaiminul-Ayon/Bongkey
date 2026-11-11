package returns

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

// PostgresStore persists tickets in PostgreSQL.
type PostgresStore struct{ DB *sql.DB }

// ticketSelect reads a ticket with the order facts staff need to call the customer and size a refund.
const ticketSelect = `
	SELECT r.id, r.order_id, o.order_number, o.customer_name, o.customer_phone, o.total, o.delivery_fee,
		r.kind, r.message, r.photos, r.status, r.outcome, r.staff_note,
		r.refund_amount, r.refund_method, r.refund_reference, r.refunded_at, r.created_at, r.closed_at
	FROM return_requests r JOIN orders o ON o.id = r.order_id`

// rowScanner is a single row or the current row of a result set.
type rowScanner interface{ Scan(dest ...any) error }

// scanTicket reads one ticketSelect row.
func scanTicket(row rowScanner) (Ticket, error) {
	var t Ticket
	var amount sql.NullInt64
	err := row.Scan(&t.ID, &t.OrderID, &t.OrderNumber, &t.CustomerName, &t.CustomerPhone, &t.OrderTotal, &t.DeliveryFee,
		&t.Kind, &t.Message, pq.Array(&t.Photos), &t.Status, &t.Outcome, &t.StaffNote,
		&amount, &t.RefundMethod, &t.RefundReference, &t.RefundedAt, &t.CreatedAt, &t.ClosedAt)
	if amount.Valid {
		value := int(amount.Int64)
		t.RefundAmount = &value
	}
	t.PhotoCount = len(t.Photos)
	return t, err
}

// Create opens a ticket; the partial unique index turns a second open ticket on the same order into ErrOpenTicket.
func (s PostgresStore) Create(ctx context.Context, orderID int, kind, message string, photos []string) (Ticket, error) {
	if photos == nil {
		photos = []string{}
	}
	var id int64
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO return_requests (order_id, kind, message, photos) VALUES ($1, $2, $3, $4) RETURNING id`,
		orderID, kind, message, pq.Array(photos)).Scan(&id)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return Ticket{}, ErrOpenTicket
	}
	if err != nil {
		return Ticket{}, err
	}
	return s.Get(ctx, id)
}

// List reads tickets, unfinished ones first and oldest first among them, so the most urgent is on top.
func (s PostgresStore) List(ctx context.Context, status string, limit int) ([]Ticket, error) {
	rows, err := s.DB.QueryContext(ctx, ticketSelect+`
		WHERE $1 = '' OR r.status = $1
		ORDER BY (r.status = 'closed'), CASE WHEN r.status = 'closed' THEN -EXTRACT(EPOCH FROM r.created_at) ELSE EXTRACT(EPOCH FROM r.created_at) END
		LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

// Get reads one ticket.
func (s PostgresStore) Get(ctx context.Context, id int64) (Ticket, error) {
	t, err := scanTicket(s.DB.QueryRowContext(ctx, ticketSelect+` WHERE r.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Ticket{}, ErrNotFound
	}
	return t, err
}

// Update changes the status of an unfinished ticket; a closed ticket stays as it ended.
func (s PostgresStore) Update(ctx context.Context, id int64, status, outcome, note string) error {
	result, err := s.DB.ExecContext(ctx, `
		UPDATE return_requests SET status = $2, outcome = $3, staff_note = $4, updated_at = NOW(),
			closed_at = CASE WHEN $2 = 'closed' THEN NOW() ELSE NULL END
		WHERE id = $1 AND status <> 'closed'`, id, status, outcome, note)
	return oneRow(result, err)
}

// RecordRefund saves the refund and closes the ticket in one statement, only while it is still unfinished;
// the table's checks refuse a refund on an exchange ticket even if the service check were skipped.
func (s PostgresStore) RecordRefund(ctx context.Context, id int64, r Refund, note string) error {
	result, err := s.DB.ExecContext(ctx, `
		UPDATE return_requests SET status = 'closed', outcome = 'refunded', refund_amount = $2, refund_method = $3,
			refund_reference = $4, refunded_at = $5, staff_note = CASE WHEN $6 = '' THEN staff_note ELSE $6 END,
			updated_at = NOW(), closed_at = NOW()
		WHERE id = $1 AND status <> 'closed' AND kind <> 'exchange'`, id, r.Amount, r.Method, r.Reference, r.PaidAt, note)
	if err := oneRow(result, err); errors.Is(err, ErrNotFound) {
		return ErrNotRefundable
	} else if err != nil {
		return err
	}
	return nil
}

// oneRow turns "no row changed" into ErrNotFound: the ticket is missing or already closed.
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
