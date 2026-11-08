package orders

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// frameRequest is a valid frame-only checkout used by the rule tests.
func frameRequest(key string) CheckoutRequest {
	fee := 80
	return CheckoutRequest{IdempotencyKey: key, CustomerName: "Rahim Uddin", CustomerPhone: "+880 1712-345678", CustomerEmail: "buyer@example.com",
		ShippingAddress: "House 7, Road 3, Agrabad", City: "Dhaka", ExpectedDeliveryFee: &fee, Items: []CheckoutItem{{ProductID: 1, Quantity: 1}}}
}

// TestCheckoutPhoneRules saves the local form of a Bangladesh mobile number and refuses anything else.
func TestCheckoutPhoneRules(t *testing.T) {
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 5}}
	service := CheckoutService{Store: &checkoutStoreStub{tx: tx}, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}
	if _, err := service.Place(context.Background(), frameRequest("checkout-phone-ok-0001")); err != nil || tx.request.CustomerPhone != "01712345678" {
		t.Fatalf("valid phone refused or not normalized: %v %q", err, tx.request.CustomerPhone)
	}
	for _, phone := range []string{"000", "01212345678", "0171234567", "+1 415 555 0100", "phone"} {
		request := frameRequest("checkout-phone-bad-0001")
		request.CustomerPhone = phone
		store := &checkoutStoreStub{tx: &checkoutTxStub{}}
		if _, err := (CheckoutService{Store: store, OrderTokenSecret: testOrderTokenSecret}).Place(context.Background(), request); !errors.Is(err, ErrInvalidPhone) || store.started {
			t.Fatalf("phone %q: err=%v started=%v", phone, err, store.started)
		}
	}
}

// TestCheckoutLimits refuses a sixth order from one phone and an eleventh from one client in the window.
func TestCheckoutLimits(t *testing.T) {
	limiter := countingLimiter{}
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 100}}
	service := CheckoutService{Store: &checkoutStoreStub{tx: tx}, EmailProofSecret: testEmailProofSecret, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80), Limiter: limiter}
	place := func(n int, phone string) error {
		request := frameRequest("checkout-limit-" + phone + "-" + string(rune('a'+n)) + "-0001")
		request.CustomerPhone, request.ClientIP = phone, "192.0.2.10"
		_, err := service.Place(context.Background(), request)
		return err
	}
	for n := 0; n < checkoutsPerPhone; n++ {
		if err := place(n, "01712345678"); err != nil {
			t.Fatalf("order %d refused: %v", n+1, err)
		}
	}
	if err := place(5, "01712345678"); !errors.Is(err, ErrCheckoutLimited) {
		t.Fatalf("sixth order from one phone: %v", err)
	}
	for n := 0; n < 4; n++ {
		if err := place(n, "01812345678"); err != nil {
			t.Fatalf("other phone refused before the client limit: %v", err)
		}
	}
	if err := place(4, "01912345678"); !errors.Is(err, ErrCheckoutLimited) {
		t.Fatalf("eleventh order from one client: %v", err)
	}
	for key := range limiter {
		if strings.Contains(key, "01712345678") || strings.Contains(key, "192.0.2.10") {
			t.Fatal("limiter key stores a raw phone or IP")
		}
	}
}

// TestRiskFlags names each unusual sign and leaves a normal order unflagged.
func TestRiskFlags(t *testing.T) {
	normal := frameRequest("")
	if flags := riskFlags(normal, 2420, PayCashOnDelivery, PhoneHistory{OrdersToday: 1}); len(flags) != 0 {
		t.Fatalf("normal order flagged: %v", flags)
	}
	bangla := normal
	bangla.CustomerName, bangla.ShippingAddress = "রহিম উদ্দিন", "বাসা ৭, রোড ৩, আগ্রাবাদ"
	if flags := riskFlags(bangla, 2420, PayCashOnDelivery, PhoneHistory{}); len(flags) != 0 {
		t.Fatalf("Bangla name and address flagged: %v", flags)
	}
	odd := normal
	odd.CustomerName, odd.CustomerEmail, odd.ShippingAddress, odd.FastSubmit = "123", "x@mailinator.com", "asdf", true
	want := []string{FlagDisposableEmail, FlagShortAddress, FlagOddName, FlagRepeatPhone, FlagPhoneOtherName, FlagHighValueCOD, FlagFastSubmit}
	if flags := riskFlags(odd, 15000, PayCashOnDelivery, PhoneHistory{OrdersToday: 2, OtherName: true}); !reflect.DeepEqual(flags, want) {
		t.Fatalf("flags = %v, want %v", flags, want)
	}
	if flags := riskFlags(normal, 15000, "advance", PhoneHistory{}); len(flags) != 0 {
		t.Fatalf("a paid-in-advance order is not a risky cash order: %v", flags)
	}
}

// TestCheckoutSavesFlags stores the flags on the new order.
func TestCheckoutSavesFlags(t *testing.T) {
	tx := &checkoutTxStub{products: map[int]ReservedProduct{1: {Name: "A", Price: 100}}, stock: map[int]int{1: 5}, history: PhoneHistory{OrdersToday: 3}}
	request := frameRequest("checkout-flags-0001")
	request.FastSubmit = true
	if _, err := (CheckoutService{Store: &checkoutStoreStub{tx: tx}, OrderTokenSecret: testOrderTokenSecret, DeliveryFeeProvider: fixedDeliveryFee(80)}).Place(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tx.flags, []string{FlagRepeatPhone, FlagFastSubmit}) {
		t.Fatalf("saved flags = %v", tx.flags)
	}
}

// TestStartOfDhakaDay uses Bangladesh midnight, which is 18:00 UTC the day before.
func TestStartOfDhakaDay(t *testing.T) {
	got := startOfDhakaDay(time.Date(2026, 11, 7, 2, 30, 0, 0, time.UTC))
	if !got.Equal(time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("start of day = %v", got.UTC())
	}
}
