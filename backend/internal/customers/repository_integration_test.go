package customers

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/security"
	_ "github.com/lib/pq"
)

// TestPostgresLinkOrders proves an email-only match is refused, email + phone links, and repeats or other owners are safe.
func TestPostgresLinkOrders(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", "020_customers.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatalf("apply customers migration: %v", err)
	}

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
