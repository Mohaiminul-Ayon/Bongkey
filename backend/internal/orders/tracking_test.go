package orders

import (
	"context"
	"errors"
	"testing"
)

// countingLimiter counts attempts per key like the shared PostgreSQL limiter.
type countingLimiter map[string]int

// Count adds one attempt for key and returns the total.
func (l countingLimiter) Count(_ context.Context, key string) (int, error) {
	l[key]++
	return l[key], nil
}

// trackingRecords is one saved order the tracking page can find by number.
func trackingRecords() memoryReader {
	order := Record{Summary: Summary{ID: 3, OrderNumber: "ORD-000123", CustomerName: "Rahim Uddin", CustomerPhone: "01712-345678",
		ShippingAddress: "House 5, Road 2", City: "Chattogram", Total: 2420, Status: "shipped",
		Tracking: Tracking{Courier: "Steadfast", TrackingCode: "SF123"}}, CustomerEmail: "rahim@example.invalid", PaymentMethod: "cash_on_delivery"}
	return memoryReader{"ORD-000123": order}
}

// TestTrackMatchesOrderNumberAndPhone shows tracking only for the right phone, in any common spelling,
// and never returns the full name, phone, address or email.
func TestTrackMatchesOrderNumberAndPhone(t *testing.T) {
	service := AccessService{Reader: trackingRecords(), Limiter: countingLimiter{}, EmailProofSecret: testEmailProofSecret}
	for _, input := range [][2]string{{"ORD-000123", "01712345678"}, {"ord-123", "+880 1712-345678"}, {" 123 ", "8801712345678"}} {
		view, err := service.Track(context.Background(), input[0], input[1], "203.0.113.5")
		if err != nil || view.OrderNumber != "ORD-000123" || view.Tracking.TrackingCode != "SF123" || len(view.Items) != 1 {
			t.Fatalf("%v: %+v %v", input, view, err)
		}
		if view.Customer != "Ra***" || view.City != "Chattogram" {
			t.Fatalf("private details leaked or mis-masked: %q %q", view.Customer, view.City)
		}
	}
	for name, input := range map[string][2]string{
		"wrong phone":     {"ORD-000123", "01799999999"},
		"short phone":     {"ORD-000123", "345678"},
		"unknown order":   {"ORD-000999", "01712345678"},
		"not a number":    {"ABC", "01712345678"},
		"numeric id form": {"0", "01712345678"},
	} {
		if _, err := service.Track(context.Background(), input[0], input[1], "203.0.113.5"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

// TestTrackIsRateLimited stops guessing after the window budget, even with the right phone.
func TestTrackIsRateLimited(t *testing.T) {
	service := AccessService{Reader: trackingRecords(), Limiter: countingLimiter{}, EmailProofSecret: testEmailProofSecret}
	for range trackAttemptsPerWindow {
		service.Track(context.Background(), "ORD-000123", "01799999999", "203.0.113.9")
	}
	if _, err := service.Track(context.Background(), "ORD-000123", "01712345678", "203.0.113.9"); !errors.Is(err, ErrTrackLimited) {
		t.Fatalf("limit not applied: %v", err)
	}
	if _, err := service.Track(context.Background(), "ORD-000123", "01712345678", "203.0.113.10"); err != nil {
		t.Fatalf("another client was limited: %v", err)
	}
	if _, err := (AccessService{Reader: trackingRecords()}).Track(context.Background(), "ORD-000123", "01712345678", "203.0.113.9"); err == nil {
		t.Fatal("tracking ran without a limiter")
	}
}

// trackingStoreStub records saved courier details.
type trackingStoreStub struct{ saved [3]string }

// SaveTracking remembers the last saved details.
func (s *trackingStoreStub) SaveTracking(_ context.Context, _ int, courier, code, link string) error {
	s.saved = [3]string{courier, code, link}
	return nil
}

// TestSetTrackingValidatesLink accepts only https links so a customer is never sent to a script or plain-http page.
func TestSetTrackingValidatesLink(t *testing.T) {
	store := &trackingStoreStub{}
	service := TrackingService{Store: store}
	if err := service.SetTracking(context.Background(), 3, " Pathao ", "PT-1", "https://merchant.pathao.com/tracking?consignment_id=PT-1"); err != nil || store.saved[0] != "Pathao" {
		t.Fatalf("valid tracking: %v %v", store.saved, err)
	}
	for name, link := range map[string]string{
		"plain http":  "http://example.com/t/1",
		"script link": "javascript:alert(1)",
		"no host":     "https:///path",
	} {
		if err := service.SetTracking(context.Background(), 3, "Pathao", "PT-1", link); !errors.Is(err, ErrInvalidTracking) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	if err := service.SetTracking(context.Background(), 0, "Pathao", "", ""); !errors.Is(err, ErrInvalidTracking) {
		t.Fatalf("bad order id accepted: %v", err)
	}
}

// TestMaskName keeps only the first two letters of the first name.
func TestMaskName(t *testing.T) {
	for in, want := range map[string]string{"Rahim Uddin": "Ra***", "A": "A***", "": "***", "জামাল হোসেন": "জা***"} {
		if got := maskName(in); got != want {
			t.Fatalf("maskName(%q) = %q, want %q", in, got, want)
		}
	}
}
