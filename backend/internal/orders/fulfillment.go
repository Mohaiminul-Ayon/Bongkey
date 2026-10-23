package orders

import (
	"context"
	"errors"
	"strings"
)

var (
	// ErrInvalidOrderStatus means the requested fulfillment state is unknown.
	ErrInvalidOrderStatus = errors.New("invalid order status")
	// ErrInvalidOrderTransition means the current order cannot move to the requested state.
	ErrInvalidOrderTransition = errors.New("invalid order status transition")
	// ErrFulfillmentConflict means a concurrent write changed the locked order unexpectedly.
	ErrFulfillmentConflict = errors.New("order fulfillment conflict")
)

// FulfillmentTx persists one status change and any required stock restoration atomically.
type FulfillmentTx interface {
	LockOrder(context.Context, int) (string, error)
	RestoreStock(context.Context, int) error
	UpdateStatus(context.Context, int, string, string) error
}

// FulfillmentStore supplies one database transaction for an order status change.
type FulfillmentStore interface {
	InTransaction(context.Context, func(FulfillmentTx) error) error
}

// FulfillmentService enforces the owner fulfillment state machine.
type FulfillmentService struct {
	Store FulfillmentStore
}

// ChangeStatus serializes one transition and restores stock once when an eligible order is cancelled.
func (s FulfillmentService) ChangeStatus(ctx context.Context, orderID int, requested string) (string, error) {
	next := strings.ToLower(strings.TrimSpace(requested))
	if orderID <= 0 || !knownOrderStatus(next) {
		return "", ErrInvalidOrderStatus
	}
	status := ""
	err := s.Store.InTransaction(ctx, func(tx FulfillmentTx) error {
		current, err := tx.LockOrder(ctx, orderID)
		if err != nil {
			return err
		}
		if current == next {
			status = current
			return nil
		}
		if !allowedOrderTransition(current, next) {
			return ErrInvalidOrderTransition
		}
		if next == "cancelled" {
			if err := tx.RestoreStock(ctx, orderID); err != nil {
				return err
			}
		}
		if err := tx.UpdateStatus(ctx, orderID, current, next); err != nil {
			return err
		}
		status = next
		return nil
	})
	return status, err
}

// knownOrderStatus bounds persisted and requested fulfillment values.
func knownOrderStatus(status string) bool {
	switch status {
	case "pending", "confirmed", "preparing", "shipped", "delivered", "cancelled":
		return true
	default:
		return false
	}
}

// allowedOrderTransition keeps return/refund work out of ordinary fulfillment.
func allowedOrderTransition(current, next string) bool {
	switch current {
	case "pending":
		return next == "confirmed" || next == "cancelled"
	case "confirmed":
		return next == "preparing" || next == "cancelled"
	case "preparing":
		return next == "shipped"
	case "shipped":
		return next == "delivered"
	default:
		return false
	}
}
