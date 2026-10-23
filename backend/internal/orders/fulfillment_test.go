package orders

import (
	"context"
	"errors"
	"testing"
)

type fulfillmentTxStub struct {
	status       string
	restoreCount int
	updateCount  int
}

// LockOrder returns the stub's current serialized state.
func (tx *fulfillmentTxStub) LockOrder(context.Context, int) (string, error) {
	return tx.status, nil
}

// RestoreStock records one requested stock restoration.
func (tx *fulfillmentTxStub) RestoreStock(context.Context, int) error {
	tx.restoreCount++
	return nil
}

// UpdateStatus records and applies the accepted transition.
func (tx *fulfillmentTxStub) UpdateStatus(_ context.Context, _ int, current, next string) error {
	if tx.status != current {
		return ErrFulfillmentConflict
	}
	tx.status = next
	tx.updateCount++
	return nil
}

type fulfillmentStoreStub struct {
	tx    *fulfillmentTxStub
	calls int
}

// InTransaction runs the service work against one serialized stub transaction.
func (store *fulfillmentStoreStub) InTransaction(_ context.Context, work func(FulfillmentTx) error) error {
	store.calls++
	return work(store.tx)
}

// TestFulfillmentTransitions checks progression, cancellation idempotency, and invalid moves.
func TestFulfillmentTransitions(t *testing.T) {
	ctx := context.Background()
	store := &fulfillmentStoreStub{tx: &fulfillmentTxStub{status: "pending"}}
	service := FulfillmentService{Store: store}
	if status, err := service.ChangeStatus(ctx, 7, "confirmed"); err != nil || status != "confirmed" || store.tx.restoreCount != 0 {
		t.Fatalf("confirmation status=%q restore=%d err=%v", status, store.tx.restoreCount, err)
	}
	if status, err := service.ChangeStatus(ctx, 7, "cancelled"); err != nil || status != "cancelled" || store.tx.restoreCount != 1 {
		t.Fatalf("cancellation status=%q restore=%d err=%v", status, store.tx.restoreCount, err)
	}
	if status, err := service.ChangeStatus(ctx, 7, "cancelled"); err != nil || status != "cancelled" || store.tx.restoreCount != 1 || store.tx.updateCount != 2 {
		t.Fatalf("repeated cancellation status=%q restore=%d updates=%d err=%v", status, store.tx.restoreCount, store.tx.updateCount, err)
	}

	store.tx.status = "preparing"
	if _, err := service.ChangeStatus(ctx, 7, "cancelled"); !errors.Is(err, ErrInvalidOrderTransition) || store.tx.restoreCount != 1 {
		t.Fatalf("invalid cancellation err=%v restore=%d", err, store.tx.restoreCount)
	}
	before := store.calls
	if _, err := service.ChangeStatus(ctx, 7, "refunded"); !errors.Is(err, ErrInvalidOrderStatus) || store.calls != before {
		t.Fatalf("unknown status reached storage: err=%v calls=%d", err, store.calls)
	}
}
