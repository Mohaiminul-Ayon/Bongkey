package orders

import (
	"context"
	"errors"
	"testing"

	"bongkey/backend/internal/security"
)

type memoryReader map[string]Record

// Find returns a synthetic order to exercise ownership without a live database.
func (r memoryReader) Find(_ context.Context, id string) (Record, error) {
	record, ok := r[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record, nil
}

// Items returns a saved line only for the synthetic legacy order.
func (r memoryReader) Items(_ context.Context, id int) ([]Item, error) {
	if id == 3 {
		return []Item{{ProductName: "Saved frame", Quantity: 1, UnitPrice: 100, TotalPrice: 100}}, nil
	}
	return []Item{}, nil
}

// TestGuestOrderAccess denies a different guest and a legacy order without a token.
func TestGuestOrderAccess(t *testing.T) {
	firstToken, firstHash, err := security.NewOrderToken()
	if err != nil {
		t.Fatal(err)
	}
	secondToken, secondHash, err := security.NewOrderToken()
	if err != nil {
		t.Fatal(err)
	}
	service := AccessService{Reader: memoryReader{
		"1": {Summary: Summary{ID: 1, CustomerPhone: "first-phone"}, GuestTokenHash: firstHash},
		"2": {Summary: Summary{ID: 2, CustomerPhone: "second-phone"}, GuestTokenHash: secondHash},
		"3": {Summary: Summary{ID: 3, CustomerPhone: "legacy-phone"}},
	}}
	order, err := service.Guest(context.Background(), "1", firstToken)
	if err != nil || order.CustomerPhone != "first-phone" {
		t.Fatalf("own order should be accessible: %+v, %v", order, err)
	}
	for _, tc := range []struct{ id, token string }{
		{"1", secondToken}, {"2", firstToken}, {"3", firstToken}, {"1", ""}, {"missing", firstToken},
	} {
		order, err := service.Guest(context.Background(), tc.id, tc.token)
		if !errors.Is(err, ErrNotFound) || order != (Summary{}) {
			t.Fatalf("guest must not receive order %s: %+v, %v", tc.id, order, err)
		}
	}
	legacy, err := service.Admin(context.Background(), "3")
	if err != nil || legacy.CustomerPhone != "legacy-phone" || len(legacy.Items) != 1 {
		t.Fatalf("admin should still access a legacy order: %+v, %v", legacy, err)
	}
}

// TestVerifiedEmailAccessRejectsOtherAndLegacyOrders checks email ownership at the service boundary.
func TestVerifiedEmailAccessRejectsOtherAndLegacyOrders(t *testing.T) {
	const secret = "synthetic-email-proof-secret-at-least-32-characters"
	proof, err := security.IssueEmailProof(secret, "buyer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	service := AccessService{EmailProofSecret: secret, Reader: memoryReader{
		"1": {Summary: Summary{ID: 1}, CustomerEmail: "buyer@example.com", EmailVerified: true},
		"2": {Summary: Summary{ID: 2}, CustomerEmail: "other@example.com", EmailVerified: true},
		"3": {Summary: Summary{ID: 3}, CustomerEmail: "buyer@example.com"},
	}}
	if order, err := service.Guest(context.Background(), "1", proof); err != nil || order.ID != 1 {
		t.Fatal("verified mailbox could not access its order")
	}
	for _, id := range []string{"2", "3", "missing"} {
		if order, err := service.Guest(context.Background(), id, proof); !errors.Is(err, ErrNotFound) || order != (Summary{}) {
			t.Fatal("email proof accessed another or unverified order")
		}
	}
}
