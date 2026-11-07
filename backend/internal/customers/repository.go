package customers

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"bongkey/backend/internal/security"
)

// PostgresStore keeps customers, their sessions and order links in PostgreSQL.
type PostgresStore struct{ DB *sql.DB }

// SaveGoogleCustomer creates the customer on first sign-in, or refreshes the email and name Google now reports.
func (p PostgresStore) SaveGoogleCustomer(ctx context.Context, identity security.GoogleIdentity) (Customer, error) {
	customer := Customer{Email: identity.Email, Name: truncate(identity.Name, 120)}
	err := p.DB.QueryRowContext(ctx, `
		INSERT INTO customers (google_sub, email, name) VALUES ($1, $2, $3)
		ON CONFLICT (google_sub) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name, last_login_at = NOW()
		RETURNING id`, identity.Subject, customer.Email, customer.Name).Scan(&customer.ID)
	return customer, err
}

// CreateSession stores a session hash and prunes a bounded batch of expired sessions.
func (p PostgresStore) CreateSession(ctx context.Context, customerID int64, tokenHash string, expires time.Time) error {
	if _, err := p.DB.ExecContext(ctx, `DELETE FROM customer_sessions WHERE token_hash IN (SELECT token_hash FROM customer_sessions WHERE expires_at < NOW() LIMIT 100)`); err != nil {
		return err
	}
	_, err := p.DB.ExecContext(ctx, `INSERT INTO customer_sessions (token_hash, customer_id, expires_at) VALUES ($1, $2, $3)`, tokenHash, customerID, expires)
	return err
}

// SessionCustomer returns the customer for an unexpired session hash, or ErrNoSession.
func (p PostgresStore) SessionCustomer(ctx context.Context, tokenHash string) (Customer, error) {
	var customer Customer
	err := p.DB.QueryRowContext(ctx, `
		SELECT c.id, c.email, c.name FROM customer_sessions s JOIN customers c ON c.id = s.customer_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW()`, tokenHash).Scan(&customer.ID, &customer.Email, &customer.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return Customer{}, ErrNoSession
	}
	return customer, err
}

// DeleteSession ends one session; deleting an unknown hash is not an error.
func (p PostgresStore) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := p.DB.ExecContext(ctx, `DELETE FROM customer_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// Orders lists the customer's linked orders, newest first.
// shortcut: capped at 100 rows with no paging; add paging if a customer ever has more.
func (p PostgresStore) Orders(ctx context.Context, customerID int64) ([]Order, error) {
	rows, err := p.DB.QueryContext(ctx, `
		SELECT id, order_number, total, order_status, created_at FROM orders
		WHERE customer_id = $1 ORDER BY created_at DESC, id DESC LIMIT 100`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Order{}
	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.OrderNumber, &order.Total, &order.Status, &order.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, order)
	}
	return list, rows.Err()
}

// LinkCandidates counts orders not linked to anyone that carry this email.
func (p PostgresStore) LinkCandidates(ctx context.Context, email string) (int, error) {
	var count int
	err := p.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE customer_id IS NULL AND LOWER(TRIM(customer_email)) = $1`, email).Scan(&count)
	return count, err
}

// LinkOrders attaches every unlinked order with this email and phone in one atomic UPDATE.
// Orders already linked (to this or another customer) are never moved, so repeating a link is harmless.
func (p PostgresStore) LinkOrders(ctx context.Context, customerID int64, email, phoneTail string) (int, error) {
	result, err := p.DB.ExecContext(ctx, `
		UPDATE orders SET customer_id = $1
		WHERE customer_id IS NULL AND LOWER(TRIM(customer_email)) = $2
		  AND RIGHT(REGEXP_REPLACE(customer_phone, '\D', '', 'g'), 10) = $3`, customerID, email, phoneTail)
	if err != nil {
		return 0, err
	}
	linked, err := result.RowsAffected()
	return int(linked), err
}

// truncate keeps at most n characters so a long Google display name cannot fail the column check.
func truncate(value string, n int) string {
	runes := []rune(value)
	if len(runes) > n {
		return string(runes[:n])
	}
	return value
}
