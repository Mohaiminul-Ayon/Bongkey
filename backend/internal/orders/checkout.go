package orders

import (
	"context"
	"errors"
	"sort"

	"bongkey/backend/internal/security"
)

const maxOrderAmount = 1<<31 - 1

var (
	// ErrInvalidCheckout means the submitted order cannot be accepted as written.
	ErrInvalidCheckout = errors.New("invalid checkout")
	// ErrUnavailable hides whether a product is missing, inactive, or out of stock.
	ErrUnavailable = errors.New("product unavailable")
)

// CheckoutItem is one requested product and lens choice.
type CheckoutItem struct {
	ProductID  int
	Quantity   int
	LensOption string
}

// CheckoutRequest contains the customer details and items needed to place a COD order.
type CheckoutRequest struct {
	CustomerName        string
	CustomerPhone       string
	CustomerEmail       string
	ShippingAddress     string
	City                string
	DeliveryNote        string
	PrescriptionFileURL string
	Items               []CheckoutItem
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
	ReserveProduct(context.Context, int, int) (ReservedProduct, error)
	InsertOrder(context.Context, CheckoutRequest, int, int, int, string) (int, string, error)
	InsertItem(context.Context, int, CheckoutItem, ReservedProduct, int) error
	InsertInvoice(context.Context, int, CheckoutRequest, int, int, int) (string, error)
}

// CheckoutStore supplies one atomic database transaction for a checkout.
type CheckoutStore interface {
	InTransaction(context.Context, func(CheckoutTx) error) error
}

// CheckoutService enforces stock and total rules before accepting an order.
type CheckoutService struct {
	Store CheckoutStore
}

// Place reserves products in a stable order and commits the order and invoice together.
func (s CheckoutService) Place(ctx context.Context, request CheckoutRequest) (CheckoutReceipt, error) {
	if request.CustomerName == "" || request.CustomerPhone == "" || request.ShippingAddress == "" || request.City == "" || len(request.Items) == 0 {
		return CheckoutReceipt{}, ErrInvalidCheckout
	}
	quantities := make(map[int]int, len(request.Items))
	for _, item := range request.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 || item.Quantity > maxOrderAmount-quantities[item.ProductID] {
			return CheckoutReceipt{}, ErrInvalidCheckout
		}
		quantities[item.ProductID] += item.Quantity
	}
	productIDs := make([]int, 0, len(quantities))
	for id := range quantities {
		productIDs = append(productIDs, id)
	}
	sort.Ints(productIDs)

	guestToken, tokenHash, err := security.NewOrderToken()
	if err != nil {
		return CheckoutReceipt{}, err
	}
	var receipt CheckoutReceipt
	err = s.Store.InTransaction(ctx, func(tx CheckoutTx) error {
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
		orderID, orderNumber, err := tx.InsertOrder(ctx, request, subtotal, deliveryFee, total, tokenHash)
		if err != nil {
			return err
		}
		for _, item := range request.Items {
			if item.LensOption == "" {
				item.LensOption = "Frame only"
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
