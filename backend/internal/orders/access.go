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
	Items           []Item
}

// Item is one accepted order line with the name and price snapshotted at checkout.
type Item struct {
	ProductName string
	LensOption  string
	Quantity    int
	UnitPrice   int
	TotalPrice  int
}

// Record keeps the access hash inside the order boundary.
type Record struct {
	Summary
	GuestTokenHash string
	CustomerEmail  string
	EmailVerified  bool
}

// Reader loads one order for the access rule without depending on HTTP.
type Reader interface {
	Find(context.Context, string) (Record, error)
}

// AccessService decides whether a guest or admin may receive private order details.
type AccessService struct {
	Reader           Reader
	EmailProofSecret string
}

// Guest returns an order for its guest token or a proof of its verified email.
func (s AccessService) Guest(ctx context.Context, id, token string) (Summary, error) {
	if token == "" {
		return Summary{}, ErrNotFound
	}
	record, err := s.Reader.Find(ctx, id)
	if err != nil {
		return Summary{}, err
	}
	if !security.MatchesOrderToken(token, record.GuestTokenHash) {
		email, err := security.VerifyEmailProof(s.EmailProofSecret, token)
		if err != nil || !record.EmailVerified || email != record.CustomerEmail {
			return Summary{}, ErrNotFound
		}
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
