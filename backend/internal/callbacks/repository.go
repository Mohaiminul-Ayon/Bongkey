package callbacks

import (
	"context"
	"database/sql"
)

// PostgresStore persists call-back requests in PostgreSQL.
type PostgresStore struct{ DB *sql.DB }

const requestColumns = `id, name, phone, product_slug, message, status, created_at, handled_at`

// Create inserts one open request and returns it as saved.
func (s PostgresStore) Create(ctx context.Context, r Request) (Request, error) {
	var saved Request
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO callback_requests (name, phone, product_slug, message) VALUES ($1, $2, $3, $4)
		RETURNING `+requestColumns, r.Name, r.Phone, r.ProductSlug, r.Message).
		Scan(&saved.ID, &saved.Name, &saved.Phone, &saved.ProductSlug, &saved.Message, &saved.Status, &saved.CreatedAt, &saved.HandledAt)
	return saved, err
}

// List reads the newest requests, optionally of one status.
func (s PostgresStore) List(ctx context.Context, status string, limit int) ([]Request, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT `+requestColumns+` FROM callback_requests
		WHERE $1 = '' OR status = $1
		ORDER BY created_at DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Request{}
	for rows.Next() {
		var r Request
		if err := rows.Scan(&r.ID, &r.Name, &r.Phone, &r.ProductSlug, &r.Message, &r.Status, &r.CreatedAt, &r.HandledAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

// MarkDone closes an open request; an already closed one keeps its first handled time.
func (s PostgresStore) MarkDone(ctx context.Context, id int64) error {
	var found bool
	err := s.DB.QueryRowContext(ctx, `
		WITH closed AS (UPDATE callback_requests SET status = 'done', handled_at = NOW() WHERE id = $1 AND status = 'open' RETURNING id)
		SELECT EXISTS (SELECT 1 FROM closed) OR EXISTS (SELECT 1 FROM callback_requests WHERE id = $1)`, id).Scan(&found)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	return nil
}
