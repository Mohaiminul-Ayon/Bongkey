package delivery

import (
	"context"
	"errors"
	"testing"
)

type settingsStoreStub struct{ fee, outsideFee int }

// Get returns the value currently held by the test store.
func (s *settingsStoreStub) Get(context.Context) (Settings, error) {
	return Settings{DeliveryFee: s.fee, OutsideDhakaFee: s.outsideFee}, nil
}

// Update records a validated fee in the test store.
func (s *settingsStoreStub) Update(_ context.Context, fee, outsideFee int) (Settings, error) {
	s.fee = fee
	s.outsideFee = outsideFee
	return Settings{DeliveryFee: fee, OutsideDhakaFee: outsideFee}, nil
}

// TestUpdateRejectsNegativeFee protects the admin input boundary.
func TestUpdateRejectsNegativeFee(t *testing.T) {
	store := &settingsStoreStub{fee: 80}
	_, err := (Service{Store: store}).Update(context.Background(), -1, 130)
	if !errors.Is(err, ErrInvalidFee) {
		t.Fatalf("expected invalid fee, got %v", err)
	}
	if store.fee != 80 {
		t.Fatalf("invalid update changed stored fee to %d", store.fee)
	}
}

// TestDeliveryFeeReadsConfiguredValue proves checkout receives the saved setting.
func TestDeliveryFeeReadsConfiguredValue(t *testing.T) {
	fee, err := (Service{Store: &settingsStoreStub{fee: 125, outsideFee: 130}}).DeliveryFee(context.Background(), "Dhaka")
	if err != nil || fee != 125 {
		t.Fatalf("expected fee 125, got %d and %v", fee, err)
	}
}

// TestDistrictPricing uses the address district, rejects unknown values, and covers every public choice.
func TestDistrictPricing(t *testing.T) {
	service := Service{Store: &settingsStoreStub{fee: 80, outsideFee: 130}}
	if len(Districts) != 64 {
		t.Fatalf("district count=%d, want 64", len(Districts))
	}
	for _, district := range Districts {
		fee, err := service.DeliveryFee(context.Background(), district)
		want := 130
		if district == "Dhaka" {
			want = 80
		}
		if err != nil || fee != want {
			t.Fatalf("district=%s fee=%d err=%v", district, fee, err)
		}
	}
	if _, err := service.DeliveryFee(context.Background(), "Dakka"); !errors.Is(err, ErrInvalidDistrict) {
		t.Fatalf("unknown district accepted: %v", err)
	}
}
