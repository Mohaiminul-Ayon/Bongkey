package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"bongkey/backend/internal/security"
)

var (
	// ErrInvalid reports invalid checkout input without exposing persistence details.
	ErrInvalid = errors.New("customer details and valid cart items are required")
	// ErrUnavailable identifies a product that cannot be purchased.
	ErrUnavailable = errors.New("cart contains unavailable product")
	// ErrCredentials indicates an absent or malformed guest credential.
	ErrCredentials = errors.New("order access token required")
	// ErrNotFound deliberately combines missing orders and denied access.
	ErrNotFound = errors.New("order not found or access denied")
)

// AccessLifetime bounds the internal-demo credential; guest recovery is a later story.
const AccessLifetime = 30 * 24 * time.Hour

// Input contains customer-supplied checkout fields, never authoritative prices.
type Input struct {
	CustomerName        string      `json:"customer_name"`
	CustomerPhone       string      `json:"customer_phone"`
	CustomerEmail       string      `json:"customer_email"`
	ShippingAddress     string      `json:"shipping_address"`
	City                string      `json:"city"`
	DeliveryNote        string      `json:"delivery_note"`
	PrescriptionFileURL string      `json:"prescription_file_url"`
	Items               []ItemInput `json:"items"`
}

// ItemInput identifies a requested item in the inherited checkout model.
type ItemInput struct {
	ProductID  int    `json:"product_id"`
	Quantity   int    `json:"quantity"`
	LensOption string `json:"lens_option"`
}

// Order is the private order summary returned only after authorization.
type Order struct {
	ID              int    `json:"id"`
	OrderNumber     string `json:"order_number"`
	InvoiceNumber   string `json:"invoice_number"`
	CustomerName    string `json:"customer_name"`
	CustomerPhone   string `json:"customer_phone"`
	ShippingAddress string `json:"shipping_address"`
	City            string `json:"city"`
	Subtotal        int    `json:"subtotal"`
	DeliveryFee     int    `json:"delivery_fee"`
	Total           int    `json:"total"`
	PaymentMethod   string `json:"payment_method"`
	OrderStatus     string `json:"order_status"`
}

// Created returns the access credential once, when checkout commits successfully.
type Created struct {
	Order
	AccessToken     string    `json:"access_token"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
}

type pricedItem struct {
	ItemInput
	Name  string
	Price int
}

// repository keeps persistence replaceable for access and rollback tests.
type repository interface {
	transact(context.Context, func(checkoutTx) error) error
	findGuest(context.Context, string, string) (Order, error)
}

type checkoutTx interface {
	product(context.Context, int) (string, int, error)
	createOrder(context.Context, Input, int, int) (int, string, error)
	addItem(context.Context, int, pricedItem) error
	createInvoice(context.Context, int, Input, int, int) (string, error)
	saveAccess(context.Context, int, string, time.Time) error
}

// Service coordinates order access and the complete checkout transaction.
type Service struct{ repo repository }

// NewService wires order use cases to the application's database connection.
func NewService(db *sql.DB) *Service { return &Service{repo: sqlRepository{db: db}} }

// GetGuest returns a summary only when this order has a matching unexpired credential.
func (s *Service) GetGuest(ctx context.Context, reference, token string) (Order, error) {
	digest, err := security.OpaqueTokenDigest(token)
	if err != nil {
		return Order{}, ErrCredentials
	}
	if len(reference) == 0 || len(reference) > 64 {
		return Order{}, ErrNotFound
	}
	order, err := s.repo.findGuest(ctx, reference, digest)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("read private order: %w", err)
	}
	return order, nil
}

// Create commits the order, invoice and hashed access grant as one atomic operation.
func (s *Service) Create(ctx context.Context, input Input) (Created, error) {
	if strings.TrimSpace(input.CustomerName) == "" || strings.TrimSpace(input.CustomerPhone) == "" || strings.TrimSpace(input.ShippingAddress) == "" || strings.TrimSpace(input.City) == "" || len(input.Items) == 0 || len(input.Items) > 100 {
		return Created{}, ErrInvalid
	}
	for _, item := range input.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 || item.Quantity > 1000 {
			return Created{}, ErrInvalid
		}
	}
	token, digest, err := security.NewOpaqueToken()
	if err != nil {
		return Created{}, fmt.Errorf("generate order access: %w", err)
	}
	expires := time.Now().UTC().Add(AccessLifetime)
	var result Created
	// The repository commits only after every operation in this callback succeeds.
	err = s.repo.transact(ctx, func(tx checkoutTx) error {
		subtotal := 0
		items := make([]pricedItem, 0, len(input.Items))
		for _, item := range input.Items {
			name, price, err := tx.product(ctx, item.ProductID)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUnavailable
			}
			if err != nil {
				return err
			}
			// PostgreSQL totals are INTEGER; reject overflow before multiplication/addition.
			if price < 0 || price > (2147483647-80-subtotal)/item.Quantity {
				return ErrInvalid
			}
			if item.LensOption == "" {
				item.LensOption = "Frame only"
			}
			subtotal += price * item.Quantity
			items = append(items, pricedItem{ItemInput: item, Name: name, Price: price})
		}
		// Preserve inherited delivery pricing until the configurable-pricing story.
		const delivery = 80
		id, number, err := tx.createOrder(ctx, input, subtotal, delivery)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := tx.addItem(ctx, id, item); err != nil {
				return err
			}
		}
		invoice, err := tx.createInvoice(ctx, id, input, subtotal, delivery)
		if err != nil {
			return err
		}
		if err := tx.saveAccess(ctx, id, digest, expires); err != nil {
			return err
		}
		result = Created{Order: Order{ID: id, OrderNumber: number, InvoiceNumber: invoice,
			CustomerName: input.CustomerName, CustomerPhone: input.CustomerPhone,
			ShippingAddress: input.ShippingAddress, City: input.City, Subtotal: subtotal,
			DeliveryFee: delivery, Total: subtotal + delivery, PaymentMethod: "cash_on_delivery", OrderStatus: "pending"},
			AccessToken: token, AccessExpiresAt: expires}
		return nil
	})
	if err != nil {
		return Created{}, fmt.Errorf("create order: %w", err)
	}
	return result, nil
}
