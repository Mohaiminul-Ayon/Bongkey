package orders

import "context"

// Advance modes: how much of an order is paid by bKash before it is confirmed, saved on each order.
// New orders use only none, delivery_fee and full; percent and fixed stay for orders saved under the
// owner's old adjustable rules, so their advance can still be recomputed after a lens price change.
const (
	AdvanceNone        = "none"
	AdvanceDeliveryFee = "delivery_fee"
	AdvancePercent     = "percent"
	AdvanceFixed       = "fixed"
	AdvanceFull        = "full"
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

// paymentFor turns the customer's choice into the order's payment terms under the owner's fixed rule:
//   - An order with any lens (mixed carts included) pays the full total first, never part of it and never
//     cash on delivery, because lenses are made to order. This holds even while bKash is off, because the
//     payment can still be arranged by phone.
//   - A frame order choosing bKash pays the full total, and only while bKash is turned on.
//   - A cash-on-delivery frame order pays the delivery charge first while bKash is on; without a way to pay
//     online nothing is required in advance.
func (s CheckoutService) paymentFor(ctx context.Context, choice string, hasLens bool) (PaymentTerms, error) {
	switch {
	case choice != "" && choice != PayCashOnDelivery && choice != PayBkash:
		return PaymentTerms{}, ErrInvalidCheckout
	case hasLens && choice == PayCashOnDelivery:
		return PaymentTerms{}, ErrInvalidCheckout
	case hasLens:
		return PaymentTerms{Method: "advance", AdvanceMode: AdvanceFull}, nil
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
	return PaymentTerms{Method: "cash_on_delivery", AdvanceMode: AdvanceDeliveryFee}, nil
}
