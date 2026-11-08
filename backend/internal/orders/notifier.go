package orders

import (
	"context"
	"database/sql"
	"errors"
)

// Notifier queues customer and owner emails after an order change has committed. Implementations
// must not fail or slow the change: they record their own errors, so callers ignore the result.
type Notifier interface {
	OrderPlaced(ctx context.Context, orderID int)
	StatusChanged(ctx context.Context, orderID int, status string)
	EmailConfirmed(ctx context.Context, orderID int)
}

// EmailConfirmationStore saves the staff tick that the customer's email was read back on the call.
type EmailConfirmationStore interface {
	SetEmailConfirmed(ctx context.Context, orderID int, confirmed bool) error
}

// EmailConfirmationService records the tick and, when set, lets the invoice email go out.
type EmailConfirmationService struct {
	Store    EmailConfirmationStore
	Notifier Notifier
}

// SetEmailConfirmed saves the tick; the invoice email waits for it because the confirmation call proves the phone, not the email.
func (s EmailConfirmationService) SetEmailConfirmed(ctx context.Context, orderID int, confirmed bool) error {
	if orderID <= 0 {
		return ErrNotFound
	}
	if err := s.Store.SetEmailConfirmed(ctx, orderID, confirmed); err != nil {
		return err
	}
	if confirmed && s.Notifier != nil {
		s.Notifier.EmailConfirmed(ctx, orderID)
	}
	return nil
}

// SetEmailConfirmed updates one order's tick; an unknown order is ErrNotFound.
func (s PostgresFulfillmentStore) SetEmailConfirmed(ctx context.Context, orderID int, confirmed bool) error {
	var id int
	err := s.DB.QueryRowContext(ctx, `UPDATE orders SET email_confirmed = $2, updated_at = NOW() WHERE id = $1 RETURNING id`, orderID, confirmed).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
