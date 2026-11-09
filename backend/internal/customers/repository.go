package customers

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"bongkey/backend/internal/security"
)

// PostgresStore keeps customers, their sessions and order links in PostgreSQL.
type PostgresStore struct{ DB *sql.DB }

// customerColumns lists the customer fields every read scans, always in this order.
const customerColumns = `id, email, name, phone`

// SaveGoogleCustomer finds the customer by Google ID, else adopts a password account with the same
// (Google-verified) email, else creates one. Google's name fills the profile only while it is empty,
// so a name the customer typed is never overwritten.
// shortcut: if Google later reports an email another account already uses, sign-in fails on the unique index; merge accounts by hand if it ever happens.
func (p PostgresStore) SaveGoogleCustomer(ctx context.Context, identity security.GoogleIdentity) (Customer, error) {
	email, name := strings.ToLower(identity.Email), truncate(identity.Name, 80)
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return Customer{}, err
	}
	defer tx.Rollback()
	var customer Customer
	err = tx.QueryRowContext(ctx, `
		UPDATE customers SET email = $2, name = CASE WHEN name = '' THEN $3 ELSE name END, last_login_at = NOW()
		WHERE google_sub = $1 RETURNING `+customerColumns, identity.Subject, email, name).Scan(&customer.ID, &customer.Email, &customer.Name, &customer.Phone)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			UPDATE customers SET google_sub = $1, name = CASE WHEN name = '' THEN $3 ELSE name END, last_login_at = NOW()
			WHERE LOWER(email) = $2 AND google_sub IS NULL RETURNING `+customerColumns, identity.Subject, email, name).Scan(&customer.ID, &customer.Email, &customer.Name, &customer.Phone)
	}
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO customers (google_sub, email, name) VALUES ($1, $2, $3)
			ON CONFLICT (google_sub) DO UPDATE SET last_login_at = NOW()
			RETURNING `+customerColumns, identity.Subject, email, name).Scan(&customer.ID, &customer.Email, &customer.Name, &customer.Phone)
	}
	if err != nil {
		return Customer{}, err
	}
	return customer, tx.Commit()
}

// PasswordCustomer reads the customer and password hash for an email; the hash is "" for a Google-only account.
func (p PostgresStore) PasswordCustomer(ctx context.Context, email string) (Customer, string, error) {
	var customer Customer
	var hash sql.NullString
	err := p.DB.QueryRowContext(ctx, `SELECT `+customerColumns+`, password_hash FROM customers WHERE LOWER(email) = $1`, email).
		Scan(&customer.ID, &customer.Email, &customer.Name, &customer.Phone, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return Customer{}, "", ErrNotFound
	}
	return customer, hash.String, err
}

// SetPassword creates the customer or replaces its password, and deletes all its sessions, in one transaction.
func (p PostgresStore) SetPassword(ctx context.Context, email, passwordHash string) (Customer, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return Customer{}, err
	}
	defer tx.Rollback()
	var customer Customer
	err = tx.QueryRowContext(ctx, `
		INSERT INTO customers (email, password_hash) VALUES ($1, $2)
		ON CONFLICT ((LOWER(email))) DO UPDATE SET password_hash = EXCLUDED.password_hash, last_login_at = NOW()
		RETURNING `+customerColumns, strings.ToLower(email), passwordHash).Scan(&customer.ID, &customer.Email, &customer.Name, &customer.Phone)
	if err != nil {
		return Customer{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM customer_sessions WHERE customer_id = $1`, customer.ID); err != nil {
		return Customer{}, err
	}
	return customer, tx.Commit()
}

// UpdateProfile saves the already-validated name and normalized phone.
func (p PostgresStore) UpdateProfile(ctx context.Context, customerID int64, name, phone string) error {
	_, err := p.DB.ExecContext(ctx, `UPDATE customers SET name = $2, phone = $3 WHERE id = $1`, customerID, name, phone)
	return err
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
		SELECT c.id, c.email, c.name, c.phone FROM customer_sessions s JOIN customers c ON c.id = s.customer_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW()`, tokenHash).Scan(&customer.ID, &customer.Email, &customer.Name, &customer.Phone)
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

// truncate keeps at most n characters so a long Google display name fits the profile name rule.
func truncate(value string, n int) string {
	runes := []rune(value)
	if len(runes) > n {
		return string(runes[:n])
	}
	return value
}
