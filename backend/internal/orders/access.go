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
	GuestTokenHash  string
	CustomerEmail   string
	EmailVerified   bool
	DeliveryNote    string
	PaymentMethod   string
	PaymentStatus   string
	CreatedAt       string
	InvoiceIssuedAt string
}

// Item is one accepted order line with its saved product name and price.
type Item struct {
	ProductName string `json:"product_name"`
	SKU         string `json:"sku"`
	LensOption  string `json:"lens_option"`
	Quantity    int    `json:"quantity"`
	UnitPrice   int    `json:"unit_price"`
	TotalPrice  int    `json:"total_price"`
}

// AdminDetail adds operational fields and item snapshots to the private order summary.
type AdminDetail struct {
	Summary
	CustomerEmail   string
	DeliveryNote    string
	PaymentMethod   string
	PaymentStatus   string
	CreatedAt       string
	InvoiceIssuedAt string
	Items           []Item
}

// Reader loads one order and its saved lines without depending on HTTP.
type Reader interface {
	Find(context.Context, string) (Record, error)
	Items(context.Context, int) ([]Item, error)
}

// AccessService decides whether a guest or admin may receive private order details.
type AccessService struct {
	Reader           Reader
	EmailProofSecret string
}

// Guest returns an order for its guest token or a proof of its verified email.
func (s AccessService) Guest(ctx context.Context, id, token string) (Summary, error) {
	record, err := s.guestRecord(ctx, id, token)
	return record.Summary, err
}

// GuestInvoice returns saved invoice lines only after order-scoped guest authorization.
func (s AccessService) GuestInvoice(ctx context.Context, id, token string) (AdminDetail, error) {
	record, err := s.guestRecord(ctx, id, token)
	if err != nil {
		return AdminDetail{}, err
	}
	return s.detail(ctx, record)
}

// guestRecord checks one guest proof against the requested order before private reads.
func (s AccessService) guestRecord(ctx context.Context, id, token string) (Record, error) {
	if token == "" {
		return Record{}, ErrNotFound
	}
	record, err := s.Reader.Find(ctx, id)
	if err != nil {
		return Record{}, err
	}
	if !security.MatchesOrderToken(token, record.GuestTokenHash) {
		email, err := security.VerifyEmailProof(s.EmailProofSecret, token)
		if err != nil || !record.EmailVerified || email != record.CustomerEmail {
			return Record{}, ErrNotFound
		}
	}
	return record, nil
}

// Admin returns an order after the admin middleware has authenticated the caller.
func (s AccessService) Admin(ctx context.Context, id string) (AdminDetail, error) {
	record, err := s.Reader.Find(ctx, id)
	if err != nil {
		return AdminDetail{}, err
	}
	return s.detail(ctx, record)
}

// detail combines one authorized order with its immutable item snapshots.
func (s AccessService) detail(ctx context.Context, record Record) (AdminDetail, error) {
	items, err := s.Reader.Items(ctx, record.ID)
	if err != nil {
		return AdminDetail{}, err
	}
	return AdminDetail{
		Summary: record.Summary, CustomerEmail: record.CustomerEmail, DeliveryNote: record.DeliveryNote,
		PaymentMethod: record.PaymentMethod, PaymentStatus: record.PaymentStatus, CreatedAt: record.CreatedAt, Items: items,
		InvoiceIssuedAt: record.InvoiceIssuedAt,
	}, nil
}
