package orders

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"bongkey/backend/internal/security"
)

type testRepository struct {
	lookup      func(context.Context, string, string) (Order, error)
	tx          *testTransaction
	commitError error
	committed   bool
}

// findGuest exposes query arguments so tests can verify the service's access boundary.
func (r *testRepository) findGuest(ctx context.Context, ref, digest string) (Order, error) {
	return r.lookup(ctx, ref, digest)
}

// transact simulates a failed operation or commit without claiming real database rollback coverage.
func (r *testRepository) transact(ctx context.Context, work func(checkoutTx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := work(r.tx); err != nil {
		return err
	}
	if r.commitError != nil {
		return r.commitError
	}
	r.committed = true
	return nil
}

type testTransaction struct {
	digest      string
	expires     time.Time
	accessError error
}

// product supplies a stable catalog price for checkout tests.
func (t *testTransaction) product(context.Context, int) (string, int, error) {
	return "Test frame", 1200, nil
}

// createOrder supplies an identifier without writing to a real database.
func (t *testTransaction) createOrder(context.Context, Input, int, int) (int, string, error) {
	return 1, "ORD-000001", nil
}

// addItem accepts the fixture item so tests can reach the access-grant boundary.
func (t *testTransaction) addItem(context.Context, int, pricedItem) error { return nil }

// createInvoice supplies an invoice identifier for the fixture order.
func (t *testTransaction) createInvoice(context.Context, int, Input, int, int) (string, error) {
	return "INV-000001", nil
}

// saveAccess captures exactly what would be persisted and can fail the transaction.
func (t *testTransaction) saveAccess(_ context.Context, _ int, digest string, expires time.Time) error {
	t.digest, t.expires = digest, expires
	return t.accessError
}

// TestGetGuestBoundary checks hashing, cancellation propagation and indistinguishable denial.
func TestGetGuestBoundary(t *testing.T) {
	token, digest, err := security.NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	repo := &testRepository{lookup: func(gotCtx context.Context, ref, hash string) (Order, error) {
		calls++
		if gotCtx != ctx || hash != digest {
			t.Fatal("lost request context or passed raw credential")
		}
		if ref != "1" {
			return Order{}, sql.ErrNoRows
		}
		return Order{ID: 1}, nil
	}}
	service := &Service{repo: repo}
	if _, err := service.GetGuest(ctx, "1", "bad"); !errors.Is(err, ErrCredentials) || calls != 0 {
		t.Fatal("malformed credential reached persistence")
	}
	if _, err := service.GetGuest(ctx, "2", token); !errors.Is(err, ErrNotFound) {
		t.Fatal("unexpected denial")
	}
	if order, err := service.GetGuest(ctx, "1", token); err != nil || order.ID != 1 {
		t.Fatal("matching credential failed")
	}
}

// TestCheckoutAccessCommit ensures credentials are returned only after a successful atomic callback and commit.
func TestCheckoutAccessCommit(t *testing.T) {
	input := Input{CustomerName: "Test Customer", CustomerPhone: "01000000000", ShippingAddress: "Test address", City: "Dhaka", Items: []ItemInput{{ProductID: 1, Quantity: 1}}}
	for _, failure := range []string{"none", "access", "commit", "invalid quantity", "cancelled"} {
		t.Run(failure, func(t *testing.T) {
			tx := &testTransaction{}
			repo := &testRepository{tx: tx}
			if failure == "access" {
				tx.accessError = errors.New("grant write failed")
			}
			if failure == "commit" {
				repo.commitError = errors.New("commit failed")
			}
			request := input
			if failure == "invalid quantity" {
				request.Items = []ItemInput{{ProductID: 1, Quantity: 0}}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "cancelled" {
				cancel()
			}
			result, err := (&Service{repo: repo}).Create(ctx, request)
			if failure != "none" {
				if err == nil || repo.committed || result.AccessToken != "" || result.ID != 0 {
					t.Fatal("failed checkout exposed a result")
				}
				return
			}
			if err != nil || !repo.committed || result.Total != 1280 {
				t.Fatalf("checkout failed: %v", err)
			}
			digest, err := security.OpaqueTokenDigest(result.AccessToken)
			if err != nil || digest != tx.digest || !result.AccessExpiresAt.Equal(tx.expires) || tx.expires.Before(time.Now()) {
				t.Fatal("incorrect access grant")
			}
		})
	}
}
