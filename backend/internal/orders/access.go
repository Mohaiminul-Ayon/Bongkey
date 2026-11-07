package orders

import (
	"context"
	"errors"

	"bongkey/backend/internal/security"
)

// ErrNotFound hides whether an order ID exists when guest access fails.
var ErrNotFound = errors.New("order not found")

// ErrInvoiceNotReady means the customer asked for the invoice before the owner confirmed the order by phone;
// until then the price or the order can still change, so the customer gets the order slip instead.
var ErrInvoiceNotReady = errors.New("invoice not ready")

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
	// LensOrder means the order has at least one lens line; such orders never use cash on delivery.
	LensOrder bool
	// PayInAdvance means something must be paid by bKash before the order is confirmed.
	PayInAdvance bool
	// AdvanceRequired is the amount due before confirmation under the order's saved rule; AmountPaid is what
	// the owner has recorded as received. The rest of the total is collected on delivery.
	AdvanceRequired int
	AmountPaid      int
	// AdvanceMode is the saved rule (none, delivery_fee, percent, fixed, full).
	AdvanceMode string
	// Tracking is when each status step happened and the courier details.
	Tracking Tracking
	// LensPricePending means the owner still has to confirm at least one lens price.
	LensPricePending bool
}

// Record keeps the access hash inside the order boundary.
type Record struct {
	Summary
	GuestTokenHash  string
	CustomerEmail   string
	EmailVerified   bool
	RiskFlags       []string
	DeliveryNote    string
	PaymentMethod   string
	PaymentStatus   string
	CreatedAt       string
	InvoiceIssuedAt string
}

// Item is one accepted order line with its saved product, lens choice and prices.
// The prescription file name stays inside the service; viewers only learn whether one exists.
type Item struct {
	ID                 int    `json:"id"`
	ProductName        string `json:"product_name"`
	SKU                string `json:"sku"`
	LensOption         string `json:"lens_option"`
	Quantity           int    `json:"quantity"`
	UnitPrice          int    `json:"unit_price"`
	TotalPrice         int    `json:"total_price"`
	LensKind           string `json:"lens_kind"`
	LensName           string `json:"lens_name"`
	LensPower          string `json:"lens_power"`
	SPHRight           *int   `json:"sph_right"`
	SPHLeft            *int   `json:"sph_left"`
	LensPrice          int    `json:"lens_price"`
	LensPriceFinal     bool   `json:"lens_price_final"`
	PrescriptionStatus string `json:"prescription_status"`
	HasPrescription    bool   `json:"has_prescription"`
	// PrescriptionNote is the owner's private review note; only admin JSON exposes items.
	PrescriptionNote string `json:"prescription_note"`
	// LensRequest is the customer's custom lens description; only admin JSON exposes items.
	LensRequest string `json:"lens_request"`
}

// AdminDetail adds operational fields and item snapshots to the private order summary.
type AdminDetail struct {
	Summary
	// RiskFlags are the red-flag codes from checkout, shown only to staff.
	RiskFlags       []string
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
	Reader Reader
	// Limiter rate-limits public tracking lookups; order pages and invoices do not use it.
	Limiter          AttemptCounter
	EmailProofSecret string
}

// Guest returns an order for its guest token or a proof of its verified email.
func (s AccessService) Guest(ctx context.Context, id, token string) (Summary, error) {
	record, err := s.guestRecord(ctx, id, token)
	return record.Summary, err
}

// GuestDetail returns the saved order lines after order-scoped guest authorization; the order slip uses it.
func (s AccessService) GuestDetail(ctx context.Context, id, token string) (AdminDetail, error) {
	record, err := s.guestRecord(ctx, id, token)
	if err != nil {
		return AdminDetail{}, err
	}
	return s.detail(ctx, record)
}

// GuestInvoice is GuestDetail for the invoice, which the customer gets only once the order is confirmed.
func (s AccessService) GuestInvoice(ctx context.Context, id, token string) (AdminDetail, error) {
	order, err := s.GuestDetail(ctx, id, token)
	if err != nil {
		return AdminDetail{}, err
	}
	switch order.Status {
	case "confirmed", "preparing", "shipped", "delivered":
		return order, nil
	default:
		return AdminDetail{}, ErrInvoiceNotReady
	}
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
		InvoiceIssuedAt: record.InvoiceIssuedAt, RiskFlags: record.RiskFlags,
	}, nil
}
