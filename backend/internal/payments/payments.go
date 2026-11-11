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

	"bongkey/backend/internal/orders"
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
	// ErrNotPayable means the order is not waiting for an advance (paid, cancelled, or nothing due before delivery).
	ErrNotPayable = errors.New("order is not waiting for a payment")
	// ErrPriceNotFinal means a lens price is still open, so the customer must not pay yet.
	ErrPriceNotFinal = errors.New("lens price not confirmed")
	// ErrMethodOff means the shop has not turned this method on.
	ErrMethodOff = errors.New("payment method is off")
	// ErrTooManyPayments stops a guest from filling the review queue.
	ErrTooManyPayments = errors.New("too many payments for this order")
	// ErrPaymentPending means an earlier transfer on the order is still waiting for the owner's check.
	ErrPaymentPending = errors.New("a payment is waiting for review")
	// ErrOverpaid means approving would record more than the order total; the owner rejects and refunds it.
	ErrOverpaid = errors.New("payment exceeds the order total")
	// ErrAlreadyReviewed means the payment was approved or rejected before.
	ErrAlreadyReviewed = errors.New("payment already reviewed")
	// ErrNotFound means no such payment or method.
	ErrNotFound = errors.New("payment not found")
)

var (
	trxPattern = regexp.MustCompile(`^[A-Z0-9]{8,12}$`)
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
	PaymentStatus string
	OrderStatus   string
	Total         int
	PriceOpen     bool
	// Required is the advance saved on the order at checkout; Paid is what has been recorded as received.
	Required int
	Paid     int
}

// checkPayable is the one rule for "may the customer send money now", used both for the customer's view
// and, under the order lock, before a submission is saved. Owner decision (#21): an unpaid balance may be
// sent by bKash until the order is delivered, so the courier collects less cash; amountDue says how much.
func checkPayable(state OrderState) error {
	if state.OrderStatus == "cancelled" || state.OrderStatus == "delivered" || state.Paid >= state.Total {
		return ErrNotPayable
	}
	if state.PriceOpen {
		return ErrPriceNotFinal
	}
	return nil
}

// checkApprovable lets the owner approve a payment the customer sent before delivery even if the order has
// since been delivered; never on a cancelled order, and never above the total (that money must be refunded).
func checkApprovable(state OrderState, amount int) error {
	switch {
	case state.OrderStatus == "cancelled":
		return ErrNotPayable
	case state.PriceOpen:
		return ErrPriceNotFinal
	case state.Paid+amount > state.Total:
		return ErrOverpaid
	}
	return nil
}

// amountDue is the exact amount one bKash transfer must carry. Owner rule (2026-11-11): no part payments,
// so while the saved advance is not covered the transfer is all of what is left of it; after that, the
// optional early payment is the whole remaining balance.
func amountDue(state OrderState) int {
	if state.Paid < state.Required {
		return state.Required - state.Paid
	}
	return state.Total - state.Paid
}

// settles reports whether approving amount covers the advance saved on the order. Until it does the order
// stays due, so a payment smaller than the agreement cannot let the order be confirmed and shipped.
func settles(state OrderState, amount int) bool {
	return state.Paid+amount >= state.Required
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
type Service struct {
	Store Store
	// Notifier may be nil; it queues the owner alert and the customer's approved or rejected email.
	Notifier Notifier
}

// Notifier queues payment emails after a change committed; implementations record their own errors.
type Notifier interface {
	PaymentSubmitted(ctx context.Context, paymentID int)
	PaymentReviewed(ctx context.Context, paymentID int)
}

// GuestView is what the customer sees for their order: whether to pay now, where, and what they sent.
type GuestView struct {
	// Payable is "yes" when the customer can pay now, otherwise the reason ("price_pending", "not_due", "off").
	Payable  string    `json:"payable"`
	Method   *Method   `json:"method,omitempty"`
	Payments []Payment `json:"payments"`
	// Required is the amount due before confirmation and Paid what is recorded; the rest is paid on delivery.
	Required int `json:"advance_required"`
	Paid     int `json:"amount_paid"`
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
	view := GuestView{Payable: "yes", Payments: list, Required: state.Required, Paid: state.Paid}
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

// Submit records a customer's transaction as "pending" after checking its shape; the store rechecks the order under lock,
// including that the amount is exactly what is due and that no other transfer is still waiting for review.
// Repeating the same transaction on the same order returns the first record instead of a second payment.
func (s Service) Submit(ctx context.Context, orderID int, sub Submission) (Payment, error) {
	sub.Method = strings.TrimSpace(sub.Method)
	sub.TrxID = strings.ToUpper(strings.TrimSpace(sub.TrxID))
	sub.SenderNumber = orders.NormalizeMobile(sub.SenderNumber)
	if orderID <= 0 || sub.Method != MethodBkash || !trxPattern.MatchString(sub.TrxID) || !orders.ValidMobile(sub.SenderNumber) || sub.Amount <= 0 || sub.Amount > maxAmount {
		return Payment{}, ErrInvalidPayment
	}
	method, err := s.Store.Method(ctx, sub.Method)
	if err != nil {
		return Payment{}, err
	}
	if !method.Enabled {
		return Payment{}, ErrMethodOff
	}
	payment, err := s.Store.Submit(ctx, orderID, sub)
	// A repeated submission returns the first record; its event key keeps the owner alert to one email.
	if err == nil && s.Notifier != nil {
		s.Notifier.PaymentSubmitted(ctx, payment.ID)
	}
	return payment, err
}

// SaveMethod updates the receiving number and instructions; an enabled method needs a valid number.
func (s Service) SaveMethod(ctx context.Context, method Method) error {
	method.AccountNumber = orders.NormalizeMobile(method.AccountNumber)
	method.Instructions = strings.TrimSpace(method.Instructions)
	if method.Code != MethodBkash || utf8.RuneCountInString(method.Instructions) > maxInstructions ||
		(method.AccountNumber != "" && !orders.ValidMobile(method.AccountNumber)) || (method.Enabled && method.AccountNumber == "") {
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

// Enabled reports whether customers can choose a method at checkout.
func (s Service) Enabled(ctx context.Context, code string) (bool, error) {
	method, err := s.Method(ctx, code)
	return method.Enabled, err
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
	if err := s.Store.Approve(ctx, paymentID, adminID); err != nil {
		return err
	}
	if s.Notifier != nil {
		s.Notifier.PaymentReviewed(ctx, paymentID)
	}
	return nil
}

// Reject closes a payment the owner could not find in the bKash app; the reason is shown to the customer.
func (s Service) Reject(ctx context.Context, paymentID int, adminID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if paymentID <= 0 || adminID <= 0 || reason == "" || utf8.RuneCountInString(reason) > maxRejectReason {
		return ErrInvalidPayment
	}
	if err := s.Store.Reject(ctx, paymentID, adminID, reason); err != nil {
		return err
	}
	if s.Notifier != nil {
		s.Notifier.PaymentReviewed(ctx, paymentID)
	}
	return nil
}
