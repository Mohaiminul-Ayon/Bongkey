package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

type seedState struct {
	count    int64
	inserted bool
	hash     string
}

type seedDriver struct{ state *seedState }

// Open supplies one fake connection for seedAdmin's SQL boundary.
func (d seedDriver) Open(string) (driver.Conn, error) { return seedConn{d.state}, nil }

type seedConn struct{ state *seedState }

// Prepare rejects unexpected statement paths in this focused test.
func (c seedConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unexpected prepare") }

// Close releases the fake connection.
func (c seedConn) Close() error { return nil }

// Begin rejects transactions without the context-aware path.
func (c seedConn) Begin() (driver.Tx, error) { return seedTx{}, nil }

// BeginTx starts the fake bootstrap transaction.
func (c seedConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) { return seedTx{}, nil }

// ExecContext records inserts and rejects any credential update.
func (c seedConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if strings.HasPrefix(query, "LOCK TABLE admins") {
		return driver.RowsAffected(0), nil
	}
	if strings.HasPrefix(query, "INSERT INTO admins") && len(args) == 3 && c.state.count == 0 {
		c.state.inserted = true
		c.state.hash = args[2].Value.(string)
		c.state.count = 1
		return driver.RowsAffected(1), nil
	}
	return nil, errors.New("unexpected admin write")
}

// QueryContext reports whether an owner already exists.
func (c seedConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if query != "SELECT COUNT(*) FROM admins" {
		return nil, errors.New("unexpected admin query")
	}
	return &seedRows{count: c.state.count}, nil
}

type seedTx struct{}

// Commit completes the fake transaction.
func (seedTx) Commit() error { return nil }

// Rollback ends an unused or failed fake transaction.
func (seedTx) Rollback() error { return nil }

type seedRows struct {
	count int64
	read  bool
}

// Columns names the single count result.
func (r *seedRows) Columns() []string { return []string{"count"} }

// Close releases the fake result.
func (r *seedRows) Close() error { return nil }

// Next returns one existing-account count.
func (r *seedRows) Next(dest []driver.Value) error {
	if r.read {
		return io.EOF
	}
	r.read = true
	dest[0] = r.count
	return nil
}

// TestSeedAdminPreservesExistingCredentials checks startup never updates an owner password.
func TestSeedAdminPreservesExistingCredentials(t *testing.T) {
	state := &seedState{count: 1}
	sql.Register("seed-admin-test", seedDriver{state})
	db, err := sql.Open("seed-admin-test", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := seedAdmin(db, "owner@example.test", "different-password"); err != nil || state.inserted {
		t.Fatalf("existing owner changed: inserted=%v err=%v", state.inserted, err)
	}
	state.count = 0
	if err := seedAdmin(db, "", ""); err == nil || state.inserted {
		t.Fatal("first owner accepted missing credentials")
	}
	if err := seedAdmin(db, "owner@example.test", "new-long-password"); err != nil || !state.inserted {
		t.Fatalf("first owner was not created: inserted=%v err=%v", state.inserted, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(state.hash), []byte("new-long-password")) != nil {
		t.Fatal("first owner password was not hashed")
	}
}
