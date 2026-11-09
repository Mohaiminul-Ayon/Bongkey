package customers

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/security"
	_ "github.com/lib/pq"
)

// localCustomersDB opens only the owner's local PostgreSQL and applies the customer migrations.
func localCustomersDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL customer QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("customer integration QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, name := range []string{"020_customers.sql", "023_customer_passwords.sql"} {
		migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	return db
}

// TestPostgresLinkOrders proves an email-only match is refused, email + phone links, and repeats or other owners are safe.
func TestPostgresLinkOrders(t *testing.T) {
	db := localCustomersDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stamp := time.Now().UnixNano()
	email := fmt.Sprintf("qa-link-%d@example.test", stamp)
	store := PostgresStore{DB: db}
	me, err := store.SaveGoogleCustomer(ctx, security.GoogleIdentity{Subject: fmt.Sprintf("qa-me-%d", stamp), Email: email, Name: "QA"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.SaveGoogleCustomer(ctx, security.GoogleIdentity{Subject: fmt.Sprintf("qa-other-%d", stamp), Email: "x" + email})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM orders WHERE order_number LIKE $1`, fmt.Sprintf("QA-LINK-%d-%%", stamp))
		_, _ = db.Exec(`DELETE FROM customers WHERE id IN ($1, $2)`, me.ID, other.ID)
	})
	insert := func(suffix, orderEmail, phone string, owner any) int64 {
		var id int64
		err := db.QueryRowContext(ctx, `
			INSERT INTO orders (order_number, customer_name, customer_phone, customer_email, shipping_address, city, subtotal, delivery_fee, total, customer_id)
			VALUES ($1, 'QA', $2, $3, 'QA address', 'Dhaka', 100, 60, 160, $4) RETURNING id`,
			fmt.Sprintf("QA-LINK-%d-%s", stamp, suffix), phone, orderEmail, owner).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	mine := insert("mine", " "+email+" ", "+880 1712-345678", nil)
	strangerTypedMyEmail := insert("stranger", email, "01898765432", nil)
	alreadyOthers := insert("others", email, "01712345678", other.ID)

	if count, err := store.LinkCandidates(ctx, email); err != nil || count != 2 {
		t.Fatalf("candidates = %d, %v; want 2 unlinked", count, err)
	}
	if linked, err := store.LinkOrders(ctx, me.ID, email, "1999999999"); err != nil || linked != 0 {
		t.Fatalf("email-only match linked %d, %v", linked, err)
	}
	if linked, err := store.LinkOrders(ctx, me.ID, email, "1712345678"); err != nil || linked != 1 {
		t.Fatalf("email + phone linked %d, %v; want 1", linked, err)
	}
	if linked, err := store.LinkOrders(ctx, me.ID, email, "1712345678"); err != nil || linked != 0 {
		t.Fatalf("repeat link linked %d, %v", linked, err)
	}
	owners := map[int64]sql.NullInt64{}
	for _, id := range []int64{mine, strangerTypedMyEmail, alreadyOthers} {
		var owner sql.NullInt64
		if err := db.QueryRowContext(ctx, `SELECT customer_id FROM orders WHERE id = $1`, id).Scan(&owner); err != nil {
			t.Fatal(err)
		}
		owners[id] = owner
	}
	if owners[mine].Int64 != me.ID || owners[strangerTypedMyEmail].Valid || owners[alreadyOthers].Int64 != other.ID {
		t.Fatalf("owners after link: %+v", owners)
	}
	list, err := store.Orders(ctx, me.ID)
	if err != nil || len(list) != 1 || list[0].ID != mine {
		t.Fatalf("my orders = %+v, %v", list, err)
	}

	hash := security.SessionTokenHash(fmt.Sprintf("qa-token-%d", stamp))
	if err := store.CreateSession(ctx, me.ID, hash, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SessionCustomer(ctx, hash); err != ErrNoSession {
		t.Fatalf("expired session: %v", err)
	}
}

// TestPostgresPasswordAccounts proves one account per email: a password sign-up, a Google sign-in with the
// same email reaching that account, a reset ending old sessions, and a typed name surviving Google sign-in.
func TestPostgresPasswordAccounts(t *testing.T) {
	db := localCustomersDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stamp := time.Now().UnixNano()
	email := fmt.Sprintf("qa-pass-%d@example.test", stamp)
	store := PostgresStore{DB: db}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM customers WHERE LOWER(email) = $1`, email) })

	first, err := store.SetPassword(ctx, email, "$2a$10$abcdefghijklmnopqrstuvabcdefghijklmnopqrstuvwxyzABCD")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateProfile(ctx, first.ID, "Typed Name", "01712345678"); err != nil {
		t.Fatal(err)
	}
	session := security.SessionTokenHash(fmt.Sprintf("qa-pass-token-%d", stamp))
	if err := store.CreateSession(ctx, first.ID, session, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	google, err := store.SaveGoogleCustomer(ctx, security.GoogleIdentity{Subject: fmt.Sprintf("qa-pass-sub-%d", stamp), Email: strings.ToUpper(email), Name: "Google Name"})
	if err != nil || google.ID != first.ID || google.Name != "Typed Name" || google.Phone != "01712345678" {
		t.Fatalf("Google sign-in for the same email = %+v, %v; want the password account with its typed profile", google, err)
	}
	again, err := store.SetPassword(ctx, strings.ToUpper(email), "$2a$10$zyxwvutsrqponmlkjihgfezyxwvutsrqponmlkjihgfedcbaZYXW")
	if err != nil || again.ID != first.ID {
		t.Fatalf("reset = %+v, %v; want the same account", again, err)
	}
	if _, err := store.SessionCustomer(ctx, session); err != ErrNoSession {
		t.Fatalf("session after reset: %v", err)
	}
	if _, hash, err := store.PasswordCustomer(ctx, email); err != nil || !strings.HasPrefix(hash, "$2a$10$zyx") {
		t.Fatalf("stored hash after reset: %q %v", hash, err)
	}
	if _, _, err := store.PasswordCustomer(ctx, "nobody-"+email); err != ErrNotFound {
		t.Fatalf("unknown email: %v", err)
	}
	if err := store.UpdateProfile(ctx, first.ID, "Typed Name", "12345"); err == nil {
		t.Fatal("database accepted a malformed phone")
	}
}
