// Package audit reads the owner's audit log. Entries are written by database triggers (022_audit_log.sql),
// in the same transaction as each change, so this package only lists them.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// pageSize bounds one page of the log.
const pageSize = 100

// Entry is one recorded change; Changes maps each changed column to {"from": old, "to": new}.
type Entry struct {
	ID        int64           `json:"id"`
	Table     string          `json:"table"`
	RecordID  string          `json:"record_id"`
	Action    string          `json:"action"`
	Changes   json.RawMessage `json:"changes"`
	CreatedAt time.Time       `json:"created_at"`
}

// Reader lists audit entries; it has no write or delete methods by design.
type Reader struct{ DB *sql.DB }

// Recent returns up to 100 entries older than beforeID (0 = newest first page), newest first.
func (r Reader) Recent(ctx context.Context, beforeID int64) ([]Entry, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, table_name, record_id, action, changes, created_at FROM audit_log
		WHERE ($1 = 0 OR id < $1) ORDER BY id DESC LIMIT $2`, beforeID, pageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []Entry{}
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.ID, &entry.Table, &entry.RecordID, &entry.Action, &entry.Changes, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
