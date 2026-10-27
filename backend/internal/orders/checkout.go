package orders

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"bongkey/backend/internal/security"
)

const maxOrderAmount = 1<<31 - 1

// frameOnlyLens is the only lens choice R0 can price; lens packages need server pricing and prescription review first.
const frameOnlyLens = "Frame only"

var (
	// ErrInvalidCheckout means the submitted order cannot be accepted as written.
	ErrInvalidCheckout = errors.New("invalid checkout")
	// ErrUnavailable hides whether a product is missing, inactive, or out of stock.
	ErrUnavailable = errors.New("product unavailable")
	// ErrIdempotencyConflict means one retry key was reused for different order data.
	ErrIdempotencyConflict = errors.New("idempotency key reused with different checkout")
)

// CheckoutItem is one requested product and lens choice.
type CheckoutItem struct {
	ProductID  int
	Quantity   int
	LensOption string
}

// CheckoutRequest contains the customer details and items needed to place a COD order.
type CheckoutRequest struct {
	IdempotencyKey  string
	CustomerName    string
	CustomerPhone   string
	CustomerEmail   string
	EmailProof      string
	EmailVerified   bool
	ShippingAddress string
	City            string
	DeliveryNote    string
	Items           []CheckoutItem
}

// CheckoutReceipt contains the accepted order totals and one-time guest token.
type CheckoutReceipt struct {
	ID               int
	OrderNumber      string
	InvoiceNumber    string
	Subtotal         int
	DeliveryFee      int
	Total            int
	GuestAccessToken string
}

// ReservedProduct contains the server-authoritative name and price of a reserved product.
type ReservedProduct struct {
	Name  string
	Price int
}

// CheckoutTx persists all pieces of an order inside one transaction.
type CheckoutTx interface {
	FindIdempotentOrder(context.Context, string) (CheckoutReceipt, string, bool, error)
	ReserveProduct(context.Context, int, int) (ReservedProduct, error)
	InsertOrder(context.Context, CheckoutRequest, int, int, int, string, string, string) (int, string, error)
	InsertItem(context.Context, int, CheckoutItem, ReservedProduct, int) error
	InsertInvoice(context.Context, int, CheckoutRequest, int, int, int) (string, error)
}

// CheckoutStore supplies one atomic database transaction for a checkout.
type CheckoutStore interface {
	InTransaction(context.Context, func(CheckoutTx) error) error
}

// CheckoutService enforces verified email, retry, stock, and total rules.
type CheckoutService struct {
	Store            CheckoutStore
	EmailProofSecret string
	OrderTokenSecret string
}

// Place reserves products in a stable order and commits the order and invoice together.
func (s CheckoutService) Place(ctx context.Context, request CheckoutRequest) (CheckoutReceipt, error) {
	if !ValidIdempotencyKey(request.IdempotencyKey) || request.CustomerName == "" || request.CustomerPhone == "" || request.ShippingAddress == "" || request.City == "" || len(request.Items) == 0 {
		return CheckoutReceipt{}, ErrInvalidCheckout
	}
	request.CustomerEmail = strings.ToLower(strings.TrimSpace(request.CustomerEmail))
	request.EmailVerified = request.CustomerEmail != ""
	quantities := make(map[int]int, len(request.Items))
	for _, item := range request.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 || item.Quantity > maxOrderAmount-quantities[item.ProductID] || (item.LensOption != "" && item.LensOption != frameOnlyLens) {
			return CheckoutReceipt{}, ErrInvalidCheckout
		}
		quantities[item.ProductID] += item.Quantity
	}
	productIDs := make([]int, 0, len(quantities))
	for id := range quantities {
		productIDs = append(productIDs, id)
	}
	sort.Ints(productIDs)

	requestHash, err := checkoutRequestHash(request)
	if err != nil {
		return CheckoutReceipt{}, err
	}
	idempotencyKeyHash := textHash(request.IdempotencyKey)
	guestToken, tokenHash, err := security.IdempotentOrderToken(s.OrderTokenSecret, request.IdempotencyKey)
	if err != nil {
		return CheckoutReceipt{}, err
	}
	var receipt CheckoutReceipt
	err = s.Store.InTransaction(ctx, func(tx CheckoutTx) error {
		existing, storedRequestHash, found, err := tx.FindIdempotentOrder(ctx, idempotencyKeyHash)
		if err != nil {
			return err
		}
		if found {
			if subtle.ConstantTimeCompare([]byte(storedRequestHash), []byte(requestHash)) != 1 {
				return ErrIdempotencyConflict
			}
			receipt = existing
			return nil
		}
		// Existing guest orders may retry their accepted key; new orders require verified email.
		if !request.EmailVerified {
			return ErrInvalidCheckout
		}
		verifiedEmail, err := security.VerifyEmailProof(s.EmailProofSecret, request.EmailProof)
		if err != nil || verifiedEmail != request.CustomerEmail {
			return ErrInvalidCheckout
		}

		products := make(map[int]ReservedProduct, len(productIDs))
		for _, id := range productIDs {
			product, err := tx.ReserveProduct(ctx, id, quantities[id])
			if err != nil {
				return err
			}
			if product.Price <= 0 {
				return ErrUnavailable
			}
			products[id] = product
		}

		const deliveryFee = 80
		subtotal := 0
		for _, item := range request.Items {
			lineTotal := int64(products[item.ProductID].Price) * int64(item.Quantity)
			if lineTotal > maxOrderAmount-deliveryFee-int64(subtotal) {
				return ErrInvalidCheckout
			}
			subtotal += int(lineTotal)
		}
		total := subtotal + deliveryFee
		orderID, orderNumber, err := tx.InsertOrder(ctx, request, subtotal, deliveryFee, total, tokenHash, idempotencyKeyHash, requestHash)
		if err != nil {
			return err
		}
		for _, item := range request.Items {
			if item.LensOption == "" {
				item.LensOption = frameOnlyLens
			}
			if err := tx.InsertItem(ctx, orderID, item, products[item.ProductID], products[item.ProductID].Price*item.Quantity); err != nil {
				return err
			}
		}
		invoiceNumber, err := tx.InsertInvoice(ctx, orderID, request, subtotal, deliveryFee, total)
		if err != nil {
			return err
		}
		receipt = CheckoutReceipt{ID: orderID, OrderNumber: orderNumber, InvoiceNumber: invoiceNumber, Subtotal: subtotal, DeliveryFee: deliveryFee, Total: total}
		return nil
	})
	if err != nil {
		return CheckoutReceipt{}, err
	}
	receipt.GuestAccessToken = guestToken
	return receipt, nil
}

// ValidIdempotencyKey accepts opaque browser-generated retry keys with bounded safe characters.
func ValidIdempotencyKey(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	for _, char := range key {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' && char != ':' {
			return false
		}
	}
	return true
}

// checkoutRequestHash fingerprints accepted business input while excluding the expiring email proof.
func checkoutRequestHash(request CheckoutRequest) (string, error) {
	// PrescriptionFileURL stays empty in the fingerprint so retries of orders accepted before it was removed still match.
	payload := struct {
		CustomerName, CustomerPhone, CustomerEmail, ShippingAddress, City, DeliveryNote, PrescriptionFileURL string
		EmailVerified                                                                                        bool
		Items                                                                                                []CheckoutItem
	}{
		CustomerName: request.CustomerName, CustomerPhone: request.CustomerPhone, CustomerEmail: request.CustomerEmail,
		ShippingAddress: request.ShippingAddress, City: request.City, DeliveryNote: request.DeliveryNote,
		EmailVerified: request.EmailVerified, Items: request.Items,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return textHash(string(encoded)), nil
}

// textHash returns a stable non-reversible database key for checkout coordination.
func textHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
