// Package payments records money customers send outside the website (bKash Send Money) and the owner's review of it.
// A submitted transaction ID is only a claim: an order counts as paid only after the owner approves it.
package payments

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// MethodBkash is the only manual method for now; the owner receives on a personal number with "Send Money".
const MethodBkash = "bkash"

const (
	// maxPaymentsPerOrder bounds submissions per order so a guest link cannot flood the owner's queue.
	maxPaymentsPerOrder = 10
	// maxAmount matches the checkout order limit.
	maxAmount        = 10_000_000
	maxInstructions  = 1000
	maxRejectReason  = 200
	StatusPending    = "pending"
	StatusApproved   = "approved"
	StatusRejected   = "rejected"
	orderAdvanceDue  = "advance_due"
	orderAdvanceDone = "advance_received"
)

var (
	// ErrInvalidPayment means a submitted or reviewed value is malformed or out of range.
	ErrInvalidPayment = errors.New("invalid payment")
	// ErrNotPayable means the order is not waiting for an advance (paid, cancelled, or not a lens order).
	ErrNotPayable = errors.New("order is not waiting for a payment")
	// ErrPriceNotFinal means a lens price is still open, so the customer must not pay yet.
	ErrPriceNotFinal = errors.New("lens price not confirmed")
	// ErrMethodOff means the shop has not turned this method on.
	ErrMethodOff = errors.New("payment method is off")
	// ErrTooManyPayments stops a guest from filling the review queue.
	ErrTooManyPayments = errors.New("too many payments for this order")
	// ErrAlreadyReviewed means the payment was approved or rejected before.
	ErrAlreadyReviewed = errors.New("payment already reviewed")
	// ErrNotFound means no such payment or method.
	ErrNotFound = errors.New("payment not found")
)

var (
	trxPattern    = regexp.MustCompile(`^[A-Z0-9]{8,12}$`)
	mobilePattern = regexp.MustCompile(`^01[3-9][0-9]{8}$`)
)

// Method is one way the shop receives money, with what the customer is told to do.
type Method struct {
	Code          string `json:"code"`
	Enabled       bool   `json:"enabled"`
	AccountNumber string `json:"account_number"`
	Instructions  string `json:"instructions"`
}

// Submission is what the customer types after sending money.
type Submission struct {
	Method       string
	TrxID        string
	SenderNumber string
	Amount       int
}

// Payment is one submitted transaction and its review.
type Payment struct {
	ID           int        `json:"id"`
	OrderID      int        `json:"order_id"`
	OrderNumber  string     `json:"order_number"`
	Method       string     `json:"method"`
	TrxID        string     `json:"trx_id"`
	SenderNumber string     `json:"sender_number"`
	Amount       int        `json:"amount"`
	Status       string     `json:"status"`
	RejectReason string     `json:"reject_reason"`
	CreatedAt    time.Time  `json:"created_at"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	// UsedOnOrders lists other orders that carry the same transaction ID; only the admin sees it.
	UsedOnOrders []string `json:"used_on_orders,omitempty"`
}

// OrderState is what decides whether an order may receive a payment.
type OrderState struct {
	PaymentMethod string
	PaymentStatus string
	OrderStatus   string
	Total         int
	PriceOpen     bool
}

// checkPayable is the one rule for "may this order take a payment now", used both for the
// customer's view and, under the order lock, before a submission or an approval is saved.
func checkPayable(state OrderState) error {
	if state.PaymentMethod != "advance" || state.PaymentStatus != orderAdvanceDue || state.OrderStatus != "pending" {
		return ErrNotPayable
	}
	if state.PriceOpen {
		return ErrPriceNotFinal
	}
	return nil
}

// checkApprovable allows approving while the advance is due, and also after an earlier part was approved,
// because a customer may send the advance in two bKash transactions.
func checkApprovable(state OrderState) error {
	if state.PaymentStatus == orderAdvanceDone {
		state.PaymentStatus = orderAdvanceDue
	}
	return checkPayable(state)
}

// Store persists methods and payments; each write that touches an order locks it and rechecks checkPayable.
type Store interface {
	Method(ctx context.Context, code string) (Method, error)
	SaveMethod(ctx context.Context, method Method) error
	OrderState(ctx context.Context, orderID int) (OrderState, error)
	Submit(ctx context.Context, orderID int, s Submission) (Payment, error)
	ForOrder(ctx context.Context, orderID int) ([]Payment, error)
	List(ctx context.Context, status string) ([]Payment, error)
	Approve(ctx context.Context, paymentID int, adminID int64) error
	Reject(ctx context.Context, paymentID int, adminID int64, reason string) error
}

// Service applies the payment rules; it never trusts a submission as money received.
type Service struct{ Store Store }

// GuestView is what the customer sees for their order: whether to pay now, where, and what they sent.
type GuestView struct {
	// Payable is "yes" when the customer can pay now, otherwise the reason ("price_pending", "not_due", "off").
	Payable  string    `json:"payable"`
	Method   *Method   `json:"method,omitempty"`
	Payments []Payment `json:"payments"`
}

// ForGuest builds the customer's payment view for an order they are already authorized to see.
// The receiving number is shown only while the order can actually be paid.
func (s Service) ForGuest(ctx context.Context, orderID int) (GuestView, error) {
	state, err := s.Store.OrderState(ctx, orderID)
	if err != nil {
		return GuestView{}, err
	}
	list, err := s.Store.ForOrder(ctx, orderID)
	if err != nil {
		return GuestView{}, err
	}
	for i := range list {
		list[i].UsedOnOrders = nil
	}
	view := GuestView{Payable: "yes", Payments: list}
	switch err := checkPayable(state); {
	case errors.Is(err, ErrPriceNotFinal):
		view.Payable = "price_pending"
		return view, nil
	case err != nil:
		view.Payable = "not_due"
		return view, nil
	}
	method, err := s.Store.Method(ctx, MethodBkash)
	if err != nil {
		return GuestView{}, err
	}
	if !method.Enabled {
		view.Payable = "off"
		return view, nil
	}
	view.Method = &method
	return view, nil
}

// Submit records a customer's transaction as "pending" after checking its shape; the store rechecks the order under lock.
// Repeating the same transaction on the same order returns the first record instead of a second payment.
func (s Service) Submit(ctx context.Context, orderID int, sub Submission) (Payment, error) {
	sub.Method = strings.TrimSpace(sub.Method)
	sub.TrxID = strings.ToUpper(strings.TrimSpace(sub.TrxID))
	sub.SenderNumber = NormalizeMobile(sub.SenderNumber)
	if orderID <= 0 || sub.Method != MethodBkash || !trxPattern.MatchString(sub.TrxID) || !mobilePattern.MatchString(sub.SenderNumber) || sub.Amount <= 0 || sub.Amount > maxAmount {
		return Payment{}, ErrInvalidPayment
	}
	method, err := s.Store.Method(ctx, sub.Method)
	if err != nil {
		return Payment{}, err
	}
	if !method.Enabled {
		return Payment{}, ErrMethodOff
	}
	return s.Store.Submit(ctx, orderID, sub)
}

// SaveMethod updates the receiving number and instructions; an enabled method needs a valid number.
func (s Service) SaveMethod(ctx context.Context, method Method) error {
	method.AccountNumber = NormalizeMobile(method.AccountNumber)
	method.Instructions = strings.TrimSpace(method.Instructions)
	if method.Code != MethodBkash || utf8.RuneCountInString(method.Instructions) > maxInstructions ||
		(method.AccountNumber != "" && !mobilePattern.MatchString(method.AccountNumber)) || (method.Enabled && method.AccountNumber == "") {
		return ErrInvalidPayment
	}
	return s.Store.SaveMethod(ctx, method)
}

// Method returns one method's settings for the owner.
func (s Service) Method(ctx context.Context, code string) (Method, error) {
	if code != MethodBkash {
		return Method{}, ErrNotFound
	}
	return s.Store.Method(ctx, code)
}

// List returns payments for the owner, newest first; status "" means every status.
func (s Service) List(ctx context.Context, status string) ([]Payment, error) {
	if status != "" && status != StatusPending && status != StatusApproved && status != StatusRejected {
		return nil, ErrInvalidPayment
	}
	return s.Store.List(ctx, status)
}

// ForOrder returns one order's payments for the owner, including reuse warnings.
func (s Service) ForOrder(ctx context.Context, orderID int) ([]Payment, error) {
	if orderID <= 0 {
		return nil, ErrNotFound
	}
	return s.Store.ForOrder(ctx, orderID)
}

// Approve records the money as received and marks the order's advance received in the same transaction.
// It is refused while a lens price is open, so the customer never pays for an unsettled price.
func (s Service) Approve(ctx context.Context, paymentID int, adminID int64) error {
	if paymentID <= 0 || adminID <= 0 {
		return ErrInvalidPayment
	}
	return s.Store.Approve(ctx, paymentID, adminID)
}

// Reject closes a payment the owner could not find in the bKash app; the reason is shown to the customer.
func (s Service) Reject(ctx context.Context, paymentID int, adminID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if paymentID <= 0 || adminID <= 0 || reason == "" || utf8.RuneCountInString(reason) > maxRejectReason {
		return ErrInvalidPayment
	}
	return s.Store.Reject(ctx, paymentID, adminID, reason)
}

// NormalizeMobile turns "+880 1712-345678" or "8801712345678" into the local 01XXXXXXXXX form; anything else is returned trimmed.
func NormalizeMobile(value string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
	if strings.HasPrefix(digits, "880") && len(digits) == 13 {
		return digits[2:]
	}
	if len(digits) == 11 {
		return digits
	}
	return strings.TrimSpace(value)
}
