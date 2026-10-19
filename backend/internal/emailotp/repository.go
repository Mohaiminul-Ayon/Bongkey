package emailotp

import (
	"context"
	"database/sql"
	"errors"
)

// PostgresStore applies cooldown, hourly limits, and single-use checks atomically.
type PostgresStore struct {
	DB *sql.DB
}

// Reserve records one request only when its IP and mailbox budgets allow it.
func (s PostgresStore) Reserve(ctx context.Context, emailKey, ipKey, codeHash string) (resultErr error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	// Small bounded cleanup keeps old mailbox/IP hashes from growing forever.
	for _, query := range []string{
		`DELETE FROM email_otp_challenges WHERE email_key IN (SELECT email_key FROM email_otp_challenges WHERE window_start < NOW() - INTERVAL '1 day' LIMIT 100)`,
		`DELETE FROM email_otp_ip_limits WHERE ip_key IN (SELECT ip_key FROM email_otp_ip_limits WHERE window_start < NOW() - INTERVAL '1 day' LIMIT 100)`,
	} {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	var ipCount int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO email_otp_ip_limits (ip_key, window_start, request_count)
		VALUES ($1, NOW(), 1)
		ON CONFLICT (ip_key) DO UPDATE SET
			request_count = CASE WHEN email_otp_ip_limits.window_start < NOW() - INTERVAL '1 hour' THEN 1 ELSE email_otp_ip_limits.request_count + 1 END,
			window_start = CASE WHEN email_otp_ip_limits.window_start < NOW() - INTERVAL '1 hour' THEN NOW() ELSE email_otp_ip_limits.window_start END
		RETURNING request_count
	`, ipKey).Scan(&ipCount)
	if err != nil {
		return err
	}
	if ipCount > 10 {
		return ErrLimited
	}
	var emailCount int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO email_otp_challenges (email_key, code_hash, expires_at, next_request_at, window_start, send_count)
		VALUES ($1, $2, NOW() + INTERVAL '5 minutes', NOW() + INTERVAL '1 minute', NOW(), 1)
		ON CONFLICT (email_key) DO UPDATE SET
			code_hash = EXCLUDED.code_hash,
			expires_at = EXCLUDED.expires_at,
			next_request_at = EXCLUDED.next_request_at,
			attempts = 0,
			consumed_at = NULL,
			send_count = CASE WHEN email_otp_challenges.window_start < NOW() - INTERVAL '1 hour' THEN 1 ELSE email_otp_challenges.send_count + 1 END,
			window_start = CASE WHEN email_otp_challenges.window_start < NOW() - INTERVAL '1 hour' THEN NOW() ELSE email_otp_challenges.window_start END
		WHERE email_otp_challenges.next_request_at <= NOW()
		  AND (email_otp_challenges.window_start < NOW() - INTERVAL '1 hour' OR email_otp_challenges.send_count < 3)
		RETURNING send_count
	`, emailKey, codeHash).Scan(&emailCount)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLimited
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Release invalidates an undelivered code while preserving abuse counters.
func (s PostgresStore) Release(ctx context.Context, emailKey, codeHash string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE email_otp_challenges SET code_hash = '', expires_at = NOW(), next_request_at = NOW() WHERE email_key = $1 AND code_hash = $2 AND consumed_at IS NULL`, emailKey, codeHash)
	return err
}

// Consume counts wrong guesses and atomically accepts a live code only once.
func (s PostgresStore) Consume(ctx context.Context, emailKey, codeHash string) (bool, error) {
	var matched bool
	err := s.DB.QueryRowContext(ctx, `
		UPDATE email_otp_challenges SET
			attempts = attempts + 1,
			consumed_at = CASE WHEN code_hash = $2 THEN NOW() ELSE consumed_at END
		WHERE email_key = $1 AND expires_at > NOW() AND consumed_at IS NULL AND attempts < 5
		RETURNING code_hash = $2
	`, emailKey, codeHash).Scan(&matched)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return matched, err
}
