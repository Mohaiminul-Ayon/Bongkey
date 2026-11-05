package orders

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"bongkey/backend/internal/security"
)

var (
	// ErrInvalidTracking means the courier details are too long or the link is not an https address.
	ErrInvalidTracking = errors.New("invalid tracking details")
	// ErrTrackingClosed means the order is not confirmed yet or was cancelled, so it has nothing to track.
	ErrTrackingClosed = errors.New("order cannot be tracked")
	// ErrTrackLimited stops one client from guessing order numbers and phones.
	ErrTrackLimited = errors.New("too many tracking attempts")
)

// trackAttemptsPerWindow allows a customer a few typos while making guessing slow (window: 15 minutes).
const trackAttemptsPerWindow = 20

// Tracking is the delivery progress shared by the customer, the public tracking page and the owner.
// The step times are set by status changes; the courier details are typed by the owner.
type Tracking struct {
	PlacedAt     time.Time  `json:"placed_at"`
	ConfirmedAt  *time.Time `json:"confirmed_at"`
	PreparingAt  *time.Time `json:"preparing_at"`
	ShippedAt    *time.Time `json:"shipped_at"`
	DeliveredAt  *time.Time `json:"delivered_at"`
	CancelledAt  *time.Time `json:"cancelled_at"`
	Courier      string     `json:"courier"`
	TrackingCode string     `json:"tracking_code"`
	TrackingURL  string     `json:"tracking_url"`
}

// TrackingStore saves the owner's courier details on an order that can still be tracked.
type TrackingStore interface {
	SaveTracking(ctx context.Context, orderID int, courier, code, link string) error
}

// TrackingService checks courier details before they are saved.
type TrackingService struct{ Store TrackingStore }

// SetTracking saves the courier name, tracking code and an optional https link for one order.
// The link is shown to customers as a clickable address, so only https links to a real host are accepted.
func (s TrackingService) SetTracking(ctx context.Context, orderID int, courier, code, link string) error {
	courier, code, link = strings.TrimSpace(courier), strings.TrimSpace(code), strings.TrimSpace(link)
	if orderID <= 0 || utf8.RuneCountInString(courier) > 60 || utf8.RuneCountInString(code) > 80 || len(link) > 300 {
		return ErrInvalidTracking
	}
	if link != "" {
		parsed, err := url.Parse(link)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return ErrInvalidTracking
		}
	}
	return s.Store.SaveTracking(ctx, orderID, courier, code, link)
}

// AttemptCounter counts one attempt for a hashed client key and returns the attempts in its window.
type AttemptCounter interface {
	Count(ctx context.Context, clientKey string) (int, error)
}

// TrackItem is one order line as the public tracking page shows it.
type TrackItem struct {
	Name       string `json:"name"`
	Option     string `json:"option"`
	Quantity   int    `json:"quantity"`
	TotalPrice int    `json:"total_price"`
}

// TrackView is what an order number and matching phone may see: no full name, phone, address, email
// or prescription details.
type TrackView struct {
	ID               int         `json:"id"`
	OrderNumber      string      `json:"order_number"`
	Status           string      `json:"order_status"`
	Tracking         Tracking    `json:"tracking"`
	Items            []TrackItem `json:"items"`
	Subtotal         int         `json:"subtotal"`
	DeliveryFee      int         `json:"delivery_fee"`
	Total            int         `json:"total"`
	AdvanceRequired  int         `json:"advance_required"`
	AmountPaid       int         `json:"amount_paid"`
	PaymentMethod    string      `json:"payment_method"`
	LensPricePending bool        `json:"lens_price_pending"`
	Customer         string      `json:"customer"`
	City             string      `json:"city"`
}

// Track finds an order by its number and the phone given at checkout. Order numbers count up, so the phone
// is the proof; every lookup is rate limited per client and a wrong number or phone gives the same ErrNotFound.
func (s AccessService) Track(ctx context.Context, orderNumber, phone, clientIP string) (TrackView, error) {
	if s.Limiter == nil || clientIP == "" {
		return TrackView{}, errors.New("order tracking limiter is not configured")
	}
	attempts, err := s.Limiter.Count(ctx, security.EmailKey(s.EmailProofSecret, "order-track-ip", clientIP))
	if err != nil {
		return TrackView{}, err
	}
	if attempts > trackAttemptsPerWindow {
		return TrackView{}, ErrTrackLimited
	}
	number, given := normalizeOrderNumber(orderNumber), phoneDigits(phone)
	if number == "" || len(given) != 11 {
		return TrackView{}, ErrNotFound
	}
	record, err := s.Reader.Find(ctx, number)
	if err != nil {
		return TrackView{}, err
	}
	if subtle.ConstantTimeCompare([]byte(phoneDigits(record.CustomerPhone)), []byte(given)) != 1 {
		return TrackView{}, ErrNotFound
	}
	items, err := s.Reader.Items(ctx, record.ID)
	if err != nil {
		return TrackView{}, err
	}
	view := TrackView{
		ID: record.ID, OrderNumber: record.OrderNumber, Status: record.Status, Tracking: record.Tracking,
		Subtotal: record.Subtotal, DeliveryFee: record.DeliveryFee, Total: record.Total,
		AdvanceRequired: record.AdvanceRequired, AmountPaid: record.AmountPaid, PaymentMethod: record.PaymentMethod,
		LensPricePending: record.LensPricePending, Customer: maskName(record.CustomerName), City: record.City,
		Items: make([]TrackItem, len(items)),
	}
	for i, item := range items {
		view.Items[i] = TrackItem{Name: item.ProductName, Option: item.LensOption, Quantity: item.Quantity, TotalPrice: item.TotalPrice}
	}
	return view, nil
}

// normalizeOrderNumber accepts "ORD-000123", "ord-123" or "123" and returns the stored form, or "" when it is not one.
func normalizeOrderNumber(value string) string {
	digits := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(value)), "ORD-")
	n, err := strconv.Atoi(digits)
	if err != nil || n <= 0 || len(digits) > 12 {
		return ""
	}
	return fmt.Sprintf("ORD-%06d", n)
}

// phoneDigits reduces "+880 1712-345678" and "01712345678" to the same 11 local digits.
func phoneDigits(value string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
	if len(digits) > 11 {
		digits = digits[len(digits)-11:]
	}
	return digits
}

// maskName shows the first two letters of the first name, e.g. "Rahim Uddin" → "Ra***".
func maskName(name string) string {
	runes := []rune(strings.TrimSpace(name))
	end := 0
	for end < len(runes) && end < 2 && !unicode.IsSpace(runes[end]) {
		end++
	}
	return string(runes[:end]) + "***"
}
