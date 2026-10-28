package orders

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"bongkey/backend/internal/delivery"
	"bongkey/backend/internal/security"
)

const (
	testOrderTokenSecret = "synthetic-order-token-secret-at-least-32-characters"
	testEmailProofSecret = "synthetic-email-proof-secret-at-least-32-characters"
)

type fixedDeliveryFee int

// DeliveryFee supplies a deterministic checkout charge for service tests.
func (fee fixedDeliveryFee) DeliveryFee(context.Context, string) (int, error) { return int(fee), nil }

type invalidDistrictFee struct{}

// DeliveryFee simulates a rejected address before checkout reserves stock.
func (invalidDistrictFee) DeliveryFee(context.Context, string) (int, error) {
	return 0, delivery.ErrInvalidDistrict
}

// TestCheckoutRejectsInvalidDistrict keeps an unknown district out of order and stock writes.
func TestCheckoutRejectsInvalidDistrict(t *testing.T) {
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 1}}
	request := verifiedCheckoutRequest(t, CheckoutRequest{IdempotencyKey: "checkout-invalid-district-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dakka", Items: []CheckoutItem{{ProductID: 1, Quantity: 1}}})
	_, err := (CheckoutService{Store: &checkoutStoreStub{tx: tx}, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: invalidDistrictFee{}}).Place(context.Background(), request)
	if !errors.Is(err, ErrInvalidCheckout) || len(tx.reservedIDs) != 0 || tx.orderSaved {
		t.Fatalf("invalid district reached persistence: %v", err)
	}
}

// verifiedCheckoutRequest adds a synthetic verified mailbox to a service fixture.
func verifiedCheckoutRequest(t *testing.T, request CheckoutRequest) CheckoutRequest {
	t.Helper()
	if request.ExpectedDeliveryFee == nil {
		fee := 80
		request.ExpectedDeliveryFee = &fee
	}
	request.CustomerEmail = "buyer@example.com"
	proof, err := security.IssueEmailProof(testEmailProofSecret, request.CustomerEmail)
	if err != nil {
		t.Fatal(err)
	}
	request.EmailProof = proof
	return request
}

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
	products                     map[int]ReservedProduct
	stock                        map[int]int
	reservedIDs                  []int
	reservedQtys                 []int
	orderSaved                   bool
	itemsSaved                   int
	invoiceSaved                 bool
	request                      CheckoutRequest
	idempotencyKeyHash           string
	idempotencyRequestHash       string
	subtotal, deliveryFee, total int
}

// FindIdempotentOrder returns the first completed stub order for matching retries.
func (tx *checkoutTxStub) FindIdempotentOrder(_ context.Context, keyHash string) (CheckoutReceipt, string, bool, error) {
	if !tx.invoiceSaved || tx.idempotencyKeyHash != keyHash {
		return CheckoutReceipt{}, "", false, nil
	}
	return CheckoutReceipt{ID: 7, OrderNumber: "ORD-000007", InvoiceNumber: "INV-000009", Subtotal: tx.subtotal, DeliveryFee: tx.deliveryFee, Total: tx.total}, tx.idempotencyRequestHash, true, nil
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
func (tx *checkoutTxStub) InsertOrder(_ context.Context, request CheckoutRequest, subtotal, deliveryFee, total int, _ string, keyHash, requestHash string) (int, string, error) {
	tx.orderSaved = true
	tx.request = request
	tx.idempotencyKeyHash = keyHash
	tx.idempotencyRequestHash = requestHash
	tx.subtotal, tx.deliveryFee, tx.total = subtotal, deliveryFee, total
	return 7, "ORD-000007", nil
}

// TestCheckoutRequiresEmailProof rejects new orders without a matching verified mailbox.
func TestCheckoutRequiresEmailProof(t *testing.T) {
	proof, err := security.IssueEmailProof(testEmailProofSecret, "buyer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	fee := 80
	base := CheckoutRequest{IdempotencyKey: "checkout-email-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka", ExpectedDeliveryFee: &fee, Items: []CheckoutItem{{ProductID: 1, Quantity: 1}}}
	for _, tc := range []struct{ email, proof string }{
		{"", ""},
		{"buyer@example.com", ""},
		{"buyer@example.com", "expired-proof"},
		{"other@example.com", proof},
		{"", proof},
	} {
		store := &checkoutStoreStub{tx: &checkoutTxStub{}}
		input := base
		input.CustomerEmail, input.EmailProof = tc.email, tc.proof
		if _, err := (CheckoutService{Store: store, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret}).Place(context.Background(), input); !errors.Is(err, ErrInvalidCheckout) || store.committed || len(store.tx.reservedIDs) != 0 {
			t.Fatal("unverified email reached stock reservation")
		}
	}
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 1}}
	input := base
	input.CustomerEmail, input.EmailProof = " Buyer@Example.com ", proof
	if _, err := (CheckoutService{Store: &checkoutStoreStub{tx: tx}, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}).Place(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if tx.request.CustomerEmail != "buyer@example.com" || !tx.request.EmailVerified {
		t.Fatal("verified email was not persisted as verified")
	}
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
	receipt, err := (CheckoutService{Store: store, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}).Place(context.Background(), verifiedCheckoutRequest(t, CheckoutRequest{
		IdempotencyKey: "checkout-stock-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 2, Quantity: 1}, {ProductID: 1, Quantity: 1}, {ProductID: 2, Quantity: 1}},
	}))
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

// TestCheckoutRequiresConfirmedDeliveryFee prevents a changed charge from silently reaching the order.
func TestCheckoutRequiresConfirmedDeliveryFee(t *testing.T) {
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 1}}
	store := &checkoutStoreStub{tx: tx}
	service := CheckoutService{Store: store, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(125)}
	request := verifiedCheckoutRequest(t, CheckoutRequest{
		IdempotencyKey: "checkout-fee-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 1, Quantity: 1}},
	})
	confirmedFee := request.ExpectedDeliveryFee
	request.ExpectedDeliveryFee = nil
	if _, err := service.Place(context.Background(), request); !errors.Is(err, ErrInvalidCheckout) {
		t.Fatalf("missing confirmed fee accepted: %v", err)
	}
	request.ExpectedDeliveryFee = confirmedFee
	service.DeliveryFeeProvider = nil
	if _, err := service.Place(context.Background(), request); !errors.Is(err, ErrDeliveryUnavailable) {
		t.Fatalf("missing fee provider accepted: %v", err)
	}
	service.DeliveryFeeProvider = fixedDeliveryFee(125)
	if _, err := service.Place(context.Background(), request); !errors.Is(err, ErrDeliveryFeeChanged) || len(tx.reservedIDs) != 0 || tx.orderSaved {
		t.Fatalf("stale fee reached stock/order: %v", err)
	}
	*request.ExpectedDeliveryFee = 125
	receipt, err := service.Place(context.Background(), request)
	if err != nil || receipt.DeliveryFee != 125 || receipt.Total != 225 || !tx.invoiceSaved {
		t.Fatalf("configured fee was not snapshotted: receipt=%+v err=%v", receipt, err)
	}
	service.DeliveryFeeProvider = fixedDeliveryFee(150)
	retry, err := service.Place(context.Background(), request)
	if err != nil || retry != receipt || len(tx.reservedIDs) != 1 {
		t.Fatalf("accepted retry changed after admin fee update: receipt=%+v err=%v", retry, err)
	}
}

// TestCheckoutIdempotencyReturnsOriginalOrder checks retry and conflicting-key behavior before stock work.
func TestCheckoutIdempotencyReturnsOriginalOrder(t *testing.T) {
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 2}}
	service := CheckoutService{Store: &checkoutStoreStub{tx: tx}, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}
	request := verifiedCheckoutRequest(t, CheckoutRequest{
		IdempotencyKey: "checkout-retry-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 1, Quantity: 1}},
	})
	first, err := service.Place(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := service.Place(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first != retry || len(tx.reservedIDs) != 1 || tx.itemsSaved != 1 {
		t.Fatalf("retry created new work: first=%+v retry=%+v reservations=%d items=%d", first, retry, len(tx.reservedIDs), tx.itemsSaved)
	}
	request.City = "Chattogram"
	if _, err := service.Place(context.Background(), request); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed payload reused checkout key: %v", err)
	}
	if len(tx.reservedIDs) != 1 || tx.itemsSaved != 1 {
		t.Fatal("conflicting retry changed stock or items")
	}
}

// TestCheckoutAllowsLegacyGuestRetry preserves the receipt for an accepted no-email order.
func TestCheckoutAllowsLegacyGuestRetry(t *testing.T) {
	request := CheckoutRequest{
		IdempotencyKey: "checkout-legacy-guest-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 1, Quantity: 1, LensOption: "Blue-cut lens"}},
	}
	requestHash, err := checkoutRequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	tx := &checkoutTxStub{invoiceSaved: true, idempotencyKeyHash: textHash(request.IdempotencyKey), idempotencyRequestHash: requestHash, subtotal: 100, deliveryFee: 80, total: 180}
	receipt, err := (CheckoutService{Store: &checkoutStoreStub{tx: tx}, OrderTokenSecret: testOrderTokenSecret}).Place(context.Background(), request)
	if err != nil || receipt.ID != 7 || receipt.GuestAccessToken == "" || len(tx.reservedIDs) != 0 {
		t.Fatalf("accepted legacy guest retry failed: receipt=%+v err=%v", receipt, err)
	}
}

// TestNewCheckoutRejectsUnpricedLenses rejects any lens other than the exact frame-only value before stock changes.
func TestNewCheckoutRejectsUnpricedLenses(t *testing.T) {
	for _, tc := range []struct {
		name, lens string
	}{
		{"blue-cut", "Blue-cut lens"},
		{"prescription", "Prescription lens"},
		{"wrong case", "frame only"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 1}}
			store := &checkoutStoreStub{tx: tx}
			request := verifiedCheckoutRequest(t, CheckoutRequest{
				IdempotencyKey: "checkout-unpriced-lens-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
				Items: []CheckoutItem{{ProductID: 1, Quantity: 1, LensOption: tc.lens}},
			})
			_, err := (CheckoutService{Store: store, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}).Place(context.Background(), request)
			if !errors.Is(err, ErrInvalidCheckout) || len(tx.reservedIDs) != 0 || tx.orderSaved || tx.invoiceSaved {
				t.Fatalf("unsupported lens reached stock/order: %v", err)
			}
		})
	}
}

// TestCheckoutRetryOutlivesEmailProof allows recovery of an accepted receipt after proof expiry.
func TestCheckoutRetryOutlivesEmailProof(t *testing.T) {
	const secret = "synthetic-email-proof-secret-at-least-32-characters"
	proof, err := security.IssueEmailProof(secret, "buyer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 1}}
	service := CheckoutService{Store: &checkoutStoreStub{tx: tx}, EmailProofSecret: secret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}
	fee := 80
	request := CheckoutRequest{
		IdempotencyKey: "checkout-email-retry-0001", CustomerName: "QA", CustomerPhone: "000", CustomerEmail: "buyer@example.com", EmailProof: proof,
		ShippingAddress: "Local", City: "Dhaka", ExpectedDeliveryFee: &fee, Items: []CheckoutItem{{ProductID: 1, Quantity: 1}},
	}
	first, err := service.Place(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.EmailProof = "expired-proof"
	retry, err := service.Place(context.Background(), request)
	if err != nil || retry != first || len(tx.reservedIDs) != 1 {
		t.Fatalf("accepted checkout could not be retried after email proof expiry: retry=%+v err=%v", retry, err)
	}
}

// TestCheckoutRejectsUnavailableStock checks that no order is accepted after stock failure.
func TestCheckoutRejectsUnavailableStock(t *testing.T) {
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 0}}
	store := &checkoutStoreStub{tx: tx}
	_, err := (CheckoutService{Store: store, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}).Place(context.Background(), verifiedCheckoutRequest(t, CheckoutRequest{
		IdempotencyKey: "checkout-stock-0002", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 1, Quantity: 1}},
	}))
	if !errors.Is(err, ErrUnavailable) || store.committed || tx.orderSaved || tx.invoiceSaved {
		t.Fatalf("unavailable stock accepted: err=%v committed=%v", err, store.committed)
	}
}

// TestCheckoutRejectsInvalidQuantity checks the request before beginning a transaction.
func TestCheckoutRejectsInvalidQuantity(t *testing.T) {
	store := &checkoutStoreStub{tx: &checkoutTxStub{}}
	_, err := (CheckoutService{Store: store, OrderTokenSecret: testOrderTokenSecret}).Place(context.Background(), CheckoutRequest{
		IdempotencyKey: "checkout-invalid-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
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
	_, err := (CheckoutService{Store: store, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}).Place(context.Background(), verifiedCheckoutRequest(t, CheckoutRequest{
		IdempotencyKey: "checkout-total-00001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 1, Quantity: 2}},
	}))
	if !errors.Is(err, ErrInvalidCheckout) || store.committed || tx.orderSaved {
		t.Fatalf("oversized amount accepted: err=%v committed=%v", err, store.committed)
	}
}

// TestCheckoutRequestHashMatchesPrePrescriptionRemoval keeps retries of already-accepted orders matching after the field removal.
func TestCheckoutRequestHashMatchesPrePrescriptionRemoval(t *testing.T) {
	request := CheckoutRequest{CustomerName: "QA", CustomerPhone: "000", CustomerEmail: "buyer@example.com", ShippingAddress: "Local", City: "Dhaka", EmailVerified: true, Items: []CheckoutItem{{ProductID: 1, Quantity: 1}}}
	got, err := checkoutRequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	// This literal is the previous fingerprint shape, which included an always-empty prescription field.
	old, err := json.Marshal(struct {
		CustomerName, CustomerPhone, CustomerEmail, ShippingAddress, City, DeliveryNote, PrescriptionFileURL string
		EmailVerified                                                                                        bool
		Items                                                                                                []CheckoutItem
	}{"QA", "000", "buyer@example.com", "Local", "Dhaka", "", "", true, request.Items})
	if err != nil {
		t.Fatal(err)
	}
	if got != textHash(string(old)) {
		t.Fatal("checkout fingerprint changed; existing idempotent retries would conflict")
	}
}
