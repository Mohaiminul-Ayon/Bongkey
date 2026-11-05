package orders

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type prescriptionSaverStub struct {
	saved   []string
	removed []string
}

// Save records one stored file name.
func (s *prescriptionSaverStub) Save([]byte) (string, error) {
	name := "file" + string(rune('a'+len(s.saved)))
	s.saved = append(s.saved, name)
	return name, nil
}

// Remove records cleanup of a file whose order did not commit.
func (s *prescriptionSaverStub) Remove(name string) { s.removed = append(s.removed, name) }

func power(v int) *int { return &v }

// lensCheckout places one line against a frame priced 1000 and the lens types used by these tests.
func lensCheckout(t *testing.T, item CheckoutItem) (*checkoutTxStub, *prescriptionSaverStub, CheckoutReceipt, error) {
	t.Helper()
	tx := &checkoutTxStub{
		products: map[int]ReservedProduct{1: {Name: "Frame", Price: 1000}},
		stock:    map[int]int{1: 5},
		lenses: map[int]LensChoice{
			7: {Name: "Blue-cut", Price: 2500, MaxPower: 600},
			8: {Name: "Power tint", Price: 3000, MaxPower: 400, PrescriptionRequired: true},
		},
	}
	saver := &prescriptionSaverStub{}
	service := CheckoutService{Store: &checkoutStoreStub{tx: tx}, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80), Prescriptions: saver}
	receipt, err := service.Place(context.Background(), verifiedCheckoutRequest(t, CheckoutRequest{
		IdempotencyKey: "checkout-lens-test-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{item},
	}))
	return tx, saver, receipt, err
}

// TestCheckoutPricesLensLines adds the server's lens price and marks it final only when the stated power is covered.
func TestCheckoutPricesLensLines(t *testing.T) {
	file := []byte("%PDF-1.7")
	for name, tc := range map[string]struct {
		item       CheckoutItem
		lensPrice  int
		final      bool
		status     string
		label      string
		savedFiles int
	}{
		"no power":           {CheckoutItem{ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensNoPower}, 2500, true, "", "Blue-cut · no power", 0},
		"power within limit": {CheckoutItem{ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensWithPower, SPHRight: power(-575), SPHLeft: power(600), Prescription: file}, 2500, true, "pending", "Blue-cut · with power", 1},
		"power above limit":  {CheckoutItem{ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensWithPower, SPHRight: power(-1600), SPHLeft: power(-200), Prescription: file}, 2500, false, "pending", "Blue-cut · with power", 1},
		"power not sure":     {CheckoutItem{ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensWithPower, SPHLeft: power(-200), Prescription: file}, 2500, false, "pending", "Blue-cut · with power", 1},
		"custom lens":        {CheckoutItem{ProductID: 1, Quantity: 1, LensKind: LensKindCustom, LensRequest: "  -16 both eyes, thin lens  "}, 0, false, "pending", "Custom lens · price confirmed by call", 0},
	} {
		tx, saver, receipt, err := lensCheckout(t, tc.item)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		line := tx.lines[0]
		if line.LensPrice != tc.lensPrice || line.LensPriceFinal != tc.final || line.PrescriptionStatus != tc.status || tx.labels[0] != tc.label {
			t.Fatalf("%s: line=%+v label=%q", name, line, tx.labels[0])
		}
		if line.LineTotal != 1000+tc.lensPrice || receipt.Total != 1000+tc.lensPrice+80 || len(saver.saved) != tc.savedFiles {
			t.Fatalf("%s: total=%d line=%d files=%d", name, receipt.Total, line.LineTotal, len(saver.saved))
		}
		if tc.item.LensKind == LensKindCustom && line.LensRequest != "-16 both eyes, thin lens" {
			t.Fatalf("%s: custom request stored as %q", name, line.LensRequest)
		}
		if tx.paymentMethod != "advance" || tx.paymentStatus != "advance_due" || receipt.PaymentMethod != "advance" {
			t.Fatalf("%s: lens order must ask for an advance, got %s/%s", name, tx.paymentMethod, tx.paymentStatus)
		}
	}
	// Two identical pairs on one line pay the frame and the lens twice and share one prescription.
	tx, saver, receipt, err := lensCheckout(t, CheckoutItem{ProductID: 1, Quantity: 2, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensWithPower, SPHRight: power(-100), SPHLeft: power(-100), Prescription: file})
	if err != nil || tx.lines[0].LineTotal != 2*(1000+2500) || receipt.Total != 2*(1000+2500)+80 || len(saver.saved) != 1 {
		t.Fatalf("two pairs: line=%+v total=%d files=%d err=%v", tx.lines, receipt.Total, len(saver.saved), err)
	}
	tx, _, _, err = lensCheckout(t, CheckoutItem{ProductID: 1, Quantity: 2})
	if err != nil || tx.paymentMethod != "cash_on_delivery" || tx.paymentStatus != "unpaid" {
		t.Fatalf("frame-only order should stay cash on delivery: %s %v", tx.paymentMethod, err)
	}
}

// TestCheckoutRejectsBadLensRequests stops lens lines that break the rules before stock or files change.
func TestCheckoutRejectsBadLensRequests(t *testing.T) {
	file := []byte("%PDF-1.7")
	for name, item := range map[string]CheckoutItem{
		"with power but no file":    {ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensWithPower},
		"no power with powers":      {ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensNoPower, SPHRight: power(-100)},
		"power off the 0.25 step":   {ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensWithPower, SPHRight: power(-110), Prescription: file},
		"power out of range":        {ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensWithPower, SPHRight: power(-2100), Prescription: file},
		"missing power choice":      {ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7},
		"unknown lens kind":         {ProductID: 1, Quantity: 1, LensKind: "free"},
		"file on a frame-only line": {ProductID: 1, Quantity: 1, Prescription: file},
		"custom with a package":     {ProductID: 1, Quantity: 1, LensKind: LensKindCustom, LensPackageID: 7, LensRequest: "thin"},
		"custom without a request":  {ProductID: 1, Quantity: 1, LensKind: LensKindCustom, LensRequest: "   "},
		"custom request too long":   {ProductID: 1, Quantity: 1, LensKind: LensKindCustom, LensRequest: strings.Repeat("x", 501)},
		"request on a package line": {ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensNoPower, LensRequest: "thin"},
		"request on a frame line":   {ProductID: 1, Quantity: 1, LensRequest: "thin"},
		"client-written lens label": {ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensNoPower, LensOption: "Free lens"},
	} {
		tx, saver, _, err := lensCheckout(t, item)
		if !errors.Is(err, ErrInvalidCheckout) || len(tx.reservedIDs) != 0 || tx.orderSaved || len(saver.saved) != 0 {
			t.Fatalf("%s: err=%v reserved=%v", name, err, tx.reservedIDs)
		}
	}
	// A type that needs a prescription cannot be bought as "no power".
	if tx, _, _, err := lensCheckout(t, CheckoutItem{ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 8, LensPower: LensNoPower}); !errors.Is(err, ErrInvalidCheckout) || tx.orderSaved {
		t.Fatalf("prescription-only type sold without power: %v", err)
	}
	// A disabled or unfitting type is refused even though the request is well formed.
	if tx, _, _, err := lensCheckout(t, CheckoutItem{ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 99, LensPower: LensNoPower}); !errors.Is(err, ErrLensUnavailable) || tx.orderSaved {
		t.Fatalf("unavailable lens type accepted: %v", err)
	}
}

// TestCheckoutRemovesFilesWhenOrderFails keeps no orphan prescriptions after a failed commit.
func TestCheckoutRemovesFilesWhenOrderFails(t *testing.T) {
	tx := &checkoutTxStub{
		products:    map[int]ReservedProduct{1: {Name: "Frame", Price: 1000}},
		stock:       map[int]int{1: 1},
		lenses:      map[int]LensChoice{7: {Name: "Blue-cut", Price: 2500, MaxPower: 600}},
		failInvoice: true,
	}
	saver := &prescriptionSaverStub{}
	service := CheckoutService{Store: &checkoutStoreStub{tx: tx}, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80), Prescriptions: saver}
	_, err := service.Place(context.Background(), verifiedCheckoutRequest(t, CheckoutRequest{
		IdempotencyKey: "checkout-lens-cleanup-0001", CustomerName: "QA", CustomerPhone: "000", ShippingAddress: "Local", City: "Dhaka",
		Items: []CheckoutItem{{ProductID: 1, Quantity: 1, LensKind: LensKindPackage, LensPackageID: 7, LensPower: LensWithPower, SPHRight: power(-100), SPHLeft: power(-100), Prescription: []byte("%PDF-1.7")}},
	}))
	if err == nil || len(saver.saved) != 1 || len(saver.removed) != 1 || saver.removed[0] != saver.saved[0] {
		t.Fatalf("failed order kept its file: err=%v saved=%v removed=%v", err, saver.saved, saver.removed)
	}
}

// TestFrameOnlyRetryFingerprintIsUnchanged keeps old frame-only retries matching after lens fields were added.
func TestFrameOnlyRetryFingerprintIsUnchanged(t *testing.T) {
	request := CheckoutRequest{CustomerName: "QA", Items: []CheckoutItem{{ProductID: 1, Quantity: 1, LensOption: "Frame only"}}}
	hash, err := checkoutRequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	// The item shape used before lens fields existed.
	type oldItem struct {
		ProductID  int
		Quantity   int
		LensOption string
	}
	old, err := json.Marshal(struct {
		CustomerName, CustomerPhone, CustomerEmail, ShippingAddress, City, DeliveryNote, PrescriptionFileURL string
		EmailVerified                                                                                        bool
		Items                                                                                                []oldItem
	}{CustomerName: "QA", Items: []oldItem{{1, 1, "Frame only"}}})
	if err != nil {
		t.Fatal(err)
	}
	if hash != textHash(string(old)) {
		t.Fatal("frame-only retry fingerprint changed; accepted retries would conflict")
	}
}
