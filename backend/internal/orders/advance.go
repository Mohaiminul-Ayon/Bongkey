package orders

import "context"

// Advance modes: how much of an order is paid by bKash before it is confirmed. The same words are used
// for the owner's settings and for the copy saved on each order.
const (
	AdvanceNone        = "none"
	AdvanceDeliveryFee = "delivery_fee"
	AdvancePercent     = "percent"
	AdvanceFixed       = "fixed"
	AdvanceFull        = "full"
)

// Advance rule kinds: frame-only orders, and orders with at least one lens line (mixed carts included).
const (
	PolicyFrame = "frame"
	PolicyLens  = "lens"
)

// PaymentTerms is the payment agreement saved on a new order.
type PaymentTerms struct {
	Method string
	Status string
	// AdvanceMode and AdvanceValue are the copy of the rule, kept so a later lens price change can
	// recompute AdvanceRequired under the same agreement.
	AdvanceMode     string
	AdvanceValue    int
	AdvanceRequired int
}

// AdvanceAmount is the one formula for the amount due before confirmation, capped at the order total.
// A percentage rounds up to whole taka, so the owner never collects less than the rule says.
func AdvanceAmount(mode string, value, total, deliveryFee int) int {
	amount := 0
	switch mode {
	case AdvanceDeliveryFee:
		amount = deliveryFee
	case AdvancePercent:
		amount = int((int64(total)*int64(value) + 99) / 100)
	case AdvanceFixed:
		amount = value
	case AdvanceFull:
		amount = total
	}
	return max(0, min(amount, total))
}

// withAmount completes the terms once the order total is known.
func (t PaymentTerms) withAmount(total, deliveryFee int) PaymentTerms {
	t.AdvanceRequired = AdvanceAmount(t.AdvanceMode, t.AdvanceValue, total, deliveryFee)
	t.Status = "unpaid"
	if t.AdvanceRequired > 0 {
		t.Status = "advance_due"
	}
	return t
}

// paymentFor turns the customer's choice and the owner's current rules into the order's payment terms.
//   - Lens orders (and mixed carts) never use cash on delivery: they follow the lens rule, even while bKash
//     is off, because the advance can still be collected by phone.
//   - A frame order choosing bKash pays the full total, and only while bKash is turned on.
//   - A cash-on-delivery frame order follows the frame rule only while bKash is on; without a way to pay
//     online nothing is required in advance.
func (s CheckoutService) paymentFor(ctx context.Context, choice string, hasLens bool) (PaymentTerms, error) {
	switch {
	case choice != "" && choice != PayCashOnDelivery && choice != PayBkash:
		return PaymentTerms{}, ErrInvalidCheckout
	case hasLens && choice == PayCashOnDelivery:
		return PaymentTerms{}, ErrInvalidCheckout
	case hasLens && s.PaymentOptions == nil:
		return PaymentTerms{Method: "advance", AdvanceMode: AdvanceFull}, nil
	case hasLens:
		mode, value, err := s.PaymentOptions.AdvancePolicy(ctx, PolicyLens)
		return PaymentTerms{Method: "advance", AdvanceMode: mode, AdvanceValue: value}, err
	case s.PaymentOptions == nil:
		if choice == PayBkash {
			return PaymentTerms{}, ErrPaymentUnavailable
		}
		return PaymentTerms{Method: "cash_on_delivery", AdvanceMode: AdvanceNone}, nil
	}
	enabled, err := s.PaymentOptions.Enabled(ctx, PayBkash)
	if err != nil {
		return PaymentTerms{}, err
	}
	switch {
	case choice == PayBkash && !enabled:
		return PaymentTerms{}, ErrPaymentUnavailable
	case choice == PayBkash:
		return PaymentTerms{Method: "advance", AdvanceMode: AdvanceFull}, nil
	case !enabled:
		return PaymentTerms{Method: "cash_on_delivery", AdvanceMode: AdvanceNone}, nil
	}
	mode, value, err := s.PaymentOptions.AdvancePolicy(ctx, PolicyFrame)
	return PaymentTerms{Method: "cash_on_delivery", AdvanceMode: mode, AdvanceValue: value}, err
}
