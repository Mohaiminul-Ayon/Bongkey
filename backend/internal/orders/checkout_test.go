package orders

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type checkoutStoreStub struct {
	tx        *checkoutTxStub
	started   bool
	committed bool
}

// InTransaction records whether the checkout reached its commit decision.
func (s *checkoutStoreStub) InTransaction(ctx context.Context, work func(CheckoutTx) error) error {
	s.started = true
	if err := work(s.tx); err != nil {
		return err
	}
	s.committed = true
	return nil
}

type checkoutTxStub struct {
	products     map[int]ReservedProduct
	stock        map[int]int
	reservedIDs  []int
	reservedQtys []int
	orderSaved   bool
	itemsSaved   int
	invoiceSaved bool
}

// ReserveProduct simulates an atomic availability check for service tests.
func (tx *checkoutTxStub) ReserveProduct(_ context.Context, id, quantity int) (ReservedProduct, error) {
	tx.reservedIDs = append(tx.reservedIDs, id)
	tx.reservedQtys = append(tx.reservedQtys, quantity)
	if tx.stock[id] < quantity {
		return ReservedProduct{}, ErrUnavailable
	}
	return tx.products[id], nil
}

// InsertOrder notes when all stock checks allowed order persistence.
func (tx *checkoutTxStub) InsertOrder(_ context.Context, _ CheckoutRequest, _, _, _ int, _ string) (int, string, error) {
	tx.orderSaved = true
	return 7, "ORD-000007", nil
}

// InsertItem counts saved lines after a successful stock reservation.
func (tx *checkoutTxStub) InsertItem(_ context.Context, _ int, _ CheckoutItem, _ ReservedProduct, _ int) error {
	tx.itemsSaved++
	return nil
}

// InsertInvoice notes when the order has a complete invoice.
func (tx *checkoutTxStub) InsertInvoice(_ context.Context, _ int, _ CheckoutRequest, _, _, _ int) (string, error) {
	tx.invoiceSaved = true
	return "INV-000009", nil
}

// TestCheckoutReservesCombinedStockInStableOrder checks duplicate lines and totals.
func TestCheckoutReservesCombinedStockInStableOrder(t *testing.T) {
	tx := &checkoutTxStub{
		products: map[int]ReservedProduct{1: {Name: "A", Price: 100}, 2: {Name: "B", Price: 200}},
		stock:    map[int]int{1: 1, 2: 2},
	}
	store := &checkoutStoreStub{tx: tx}
	receipt, err := (CheckoutService{Store: store}).Place(context.Background(), CheckoutRequest{
		CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 2, Quantity: 1}, {ProductID: 1, Quantity: 1}, {ProductID: 2, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tx.reservedIDs, []int{1, 2}) || !reflect.DeepEqual(tx.reservedQtys, []int{1, 2}) {
		t.Fatalf("reservations = %v quantities = %v", tx.reservedIDs, tx.reservedQtys)
	}
	if !store.committed || !tx.orderSaved || !tx.invoiceSaved || tx.itemsSaved != 3 {
		t.Fatal("successful checkout did not save every piece")
	}
	if receipt.Subtotal != 500 || receipt.DeliveryFee != 80 || receipt.Total != 580 || receipt.GuestAccessToken == "" {
		t.Fatalf("unexpected checkout receipt: %+v", receipt)
	}
}

// TestCheckoutRejectsUnavailableStock checks that no order is accepted after stock failure.
func TestCheckoutRejectsUnavailableStock(t *testing.T) {
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 0}}
	store := &checkoutStoreStub{tx: tx}
	_, err := (CheckoutService{Store: store}).Place(context.Background(), CheckoutRequest{
		CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 1, Quantity: 1}},
	})
	if !errors.Is(err, ErrUnavailable) || store.committed || tx.orderSaved || tx.invoiceSaved {
		t.Fatalf("unavailable stock accepted: err=%v committed=%v", err, store.committed)
	}
}

// TestCheckoutRejectsInvalidQuantity checks the request before beginning a transaction.
func TestCheckoutRejectsInvalidQuantity(t *testing.T) {
	store := &checkoutStoreStub{tx: &checkoutTxStub{}}
	_, err := (CheckoutService{Store: store}).Place(context.Background(), CheckoutRequest{
		CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 1, Quantity: 0}},
	})
	if !errors.Is(err, ErrInvalidCheckout) || store.started {
		t.Fatalf("invalid quantity started checkout: err=%v started=%v", err, store.started)
	}
}

// TestCheckoutRejectsOversizedTotal checks integer order totals before persistence.
func TestCheckoutRejectsOversizedTotal(t *testing.T) {
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: maxOrderAmount}}, stock: map[int]int{1: 2}}
	store := &checkoutStoreStub{tx: tx}
	_, err := (CheckoutService{Store: store}).Place(context.Background(), CheckoutRequest{
		CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 1, Quantity: 2}},
	})
	if !errors.Is(err, ErrInvalidCheckout) || store.committed || tx.orderSaved {
		t.Fatalf("oversized amount accepted: err=%v committed=%v", err, store.committed)
	}
}
