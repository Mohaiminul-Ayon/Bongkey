package orders

import (
	"context"
	"errors"

	"bongkey/backend/internal/security"
)

// ErrNotFound hides whether an order ID exists when guest access fails.
var ErrNotFound = errors.New("order not found")

// Summary contains the order details returned to an authorized viewer.
type Summary struct {
	ID              int
	OrderNumber     string
	InvoiceNumber   string
	CustomerName    string
	CustomerPhone   string
	ShippingAddress string
	City            string
	Subtotal        int
	DeliveryFee     int
	Total           int
	Status          string
}

// Record keeps the access hash inside the order boundary.
type Record struct {
	Summary
	GuestTokenHash string
}

// Reader loads one order for the access rule without depending on HTTP.
type Reader interface {
	Find(context.Context, string) (Record, error)
}

// AccessService decides whether a guest or admin may receive private order details.
type AccessService struct {
	Reader Reader
}

// Guest returns an order only when its guest token matches the stored hash.
func (s AccessService) Guest(ctx context.Context, id, token string) (Summary, error) {
	if token == "" {
		return Summary{}, ErrNotFound
	}
	record, err := s.Reader.Find(ctx, id)
	if err != nil {
		return Summary{}, err
	}
	if !security.MatchesOrderToken(token, record.GuestTokenHash) {
		return Summary{}, ErrNotFound
	}
	return record.Summary, nil
}

// Admin returns an order after the admin middleware has authenticated the caller.
func (s AccessService) Admin(ctx context.Context, id string) (Summary, error) {
	record, err := s.Reader.Find(ctx, id)
	if err != nil {
		return Summary{}, err
	}
	return record.Summary, nil
}
