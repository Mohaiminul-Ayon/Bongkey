package delivery

import (
	"context"
	"errors"
	"strings"
)

const maxDeliveryFee = 1000000

// ErrInvalidFee rejects charges outside the supported money range.
var ErrInvalidFee = errors.New("invalid delivery fee")

// ErrInvalidDistrict rejects an address outside the listed Bangladesh districts.
var ErrInvalidDistrict = errors.New("invalid district")

// Settings is the global delivery configuration used by checkout and admin settings.
type Settings struct {
	DeliveryFee     int      `json:"delivery_fee"`
	OutsideDhakaFee int      `json:"outside_dhaka_fee"`
	Districts       []string `json:"districts,omitempty"`
}

// Store persists and reads the single global delivery setting.
type Store interface {
	Get(context.Context) (Settings, error)
	Update(context.Context, int, int) (Settings, error)
}

// Service validates delivery settings before they are persisted.
type Service struct{ Store Store }

// Get returns the currently configured global delivery charge.
func (s Service) Get(ctx context.Context) (Settings, error) {
	settings, err := s.Store.Get(ctx)
	settings.Districts = Districts
	return settings, err
}

// Update validates and saves the global delivery charge.
func (s Service) Update(ctx context.Context, fee, outsideFee int) (Settings, error) {
	if fee < 0 || fee > maxDeliveryFee || outsideFee < 0 || outsideFee > maxDeliveryFee {
		return Settings{}, ErrInvalidFee
	}
	settings, err := s.Store.Update(ctx, fee, outsideFee)
	settings.Districts = Districts
	return settings, err
}

// DeliveryFee reads the configured fee for checkout.
func (s Service) DeliveryFee(ctx context.Context, district string) (int, error) {
	if !ValidDistrict(district) {
		return 0, ErrInvalidDistrict
	}
	settings, err := s.Store.Get(ctx)
	if err != nil {
		return 0, err
	}
	if settings.DeliveryFee < 0 || settings.DeliveryFee > maxDeliveryFee || settings.OutsideDhakaFee < 0 || settings.OutsideDhakaFee > maxDeliveryFee {
		return 0, ErrInvalidFee
	}
	if !strings.EqualFold(strings.TrimSpace(district), "Dhaka") {
		return settings.OutsideDhakaFee, nil
	}
	return settings.DeliveryFee, nil
}
