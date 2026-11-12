// Package returns handles return and exchange tickets after delivery: the customer describes the problem,
// staff call or chat and agree a solution, then close the ticket and record any refund actually paid.
package returns

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Ticket kinds. Owner rules (2026-11-12): a change of mind is an exchange for another frame within 3 days of
// delivery; a defect, wrong colour or wrong item may be reported within 15 days. Only problems are refunded.
const (
	KindExchange  = "exchange"
	KindDefect    = "defect"
	KindWrongItem = "wrong_item"
)

// Ticket statuses and the outcomes a closed ticket records.
const (
	StatusOpen       = "open"
	StatusInProgress = "in_progress"
	StatusClosed     = "closed"

	OutcomeExchanged = "exchanged"
	OutcomeReplaced  = "replaced"
	OutcomeRefunded  = "refunded"
	OutcomeDeclined  = "declined"
)

const (
	day            = 24 * time.Hour
	exchangeWindow = 3 * day
	problemWindow  = 15 * day
	// RefundDeadline counts from the request, not from approval (PRD 10.1), so slow review cannot stretch it.
	RefundDeadline = 15 * day
	// MaxPhotos bounds one ticket's pictures; each file is checked by the photo store.
	MaxPhotos              = 3
	minMessage, maxMessage = 5, 1000
	maxNote                = 1000
	maxReference           = 60
)

var (
	// ErrInvalid means a field is missing or malformed.
	ErrInvalid = errors.New("invalid return request")
	// ErrNotDelivered means the order has not been delivered (or was cancelled), so there is nothing to return yet.
	ErrNotDelivered = errors.New("order not delivered")
	// ErrWindowClosed means the time for this kind of request is over.
	ErrWindowClosed = errors.New("return window closed")
	// ErrOpenTicket means the order already has a ticket staff are working on.
	ErrOpenTicket = errors.New("order already has an open ticket")
	// ErrNotFound means no ticket (or photo) has that ID.
	ErrNotFound = errors.New("return request not found")
	// ErrNotRefundable means the ticket is an exchange or already closed, so no refund can be recorded.
	ErrNotRefundable = errors.New("ticket cannot be refunded")
)

// Order is what the caller has already authorized and read: the order, its state and when it was delivered.
type Order struct {
	ID          int
	Status      string
	DeliveredAt *time.Time
}

// Submission is the customer's request; Photos are raw file contents, checked by the photo store.
type Submission struct {
	Kind    string
	Message string
	Photos  [][]byte
}

// Ticket is one return or exchange request as staff see it.
type Ticket struct {
	ID              int64      `json:"id"`
	OrderID         int        `json:"order_id"`
	OrderNumber     string     `json:"order_number"`
	CustomerName    string     `json:"customer_name"`
	CustomerPhone   string     `json:"customer_phone"`
	OrderTotal      int        `json:"order_total"`
	DeliveryFee     int        `json:"delivery_fee"`
	Kind            string     `json:"kind"`
	Message         string     `json:"message"`
	Photos          []string   `json:"-"`
	PhotoCount      int        `json:"photo_count"`
	Status          string     `json:"status"`
	Outcome         string     `json:"outcome"`
	StaffNote       string     `json:"staff_note"`
	RefundAmount    *int       `json:"refund_amount"`
	RefundMethod    string     `json:"refund_method"`
	RefundReference string     `json:"refund_reference"`
	RefundedAt      *time.Time `json:"refunded_at"`
	CreatedAt       time.Time  `json:"created_at"`
	ClosedAt        *time.Time `json:"closed_at"`
	// RefundDue is the latest date a refund may be paid (problem tickets only); Overdue means it passed while open.
	RefundDue *time.Time `json:"refund_due"`
	Overdue   bool       `json:"overdue"`
}

// Refund is money actually sent back to the customer.
type Refund struct {
	Amount    int
	Method    string
	Reference string
	PaidAt    time.Time
}

// Store persists tickets.
type Store interface {
	Create(ctx context.Context, orderID int, kind, message string, photos []string) (Ticket, error)
	List(ctx context.Context, status string, limit int) ([]Ticket, error)
	Get(ctx context.Context, id int64) (Ticket, error)
	Update(ctx context.Context, id int64, status, outcome, note string) error
	RecordRefund(ctx context.Context, id int64, r Refund, note string) error
}

// PhotoStore keeps the customer's photos privately; lens.PrescriptionStore implements it.
type PhotoStore interface {
	Save(data []byte) (string, error)
	Remove(name string)
	Path(name string) (string, error)
}

// Notifier queues the owner's alert; notifications.Service implements it and records its own errors.
type Notifier interface {
	ReturnRequested(ctx context.Context, t Ticket)
}

// Service applies the return rules. Notifier may be nil; Now defaults to time.Now and is replaced in tests.
type Service struct {
	Store    Store
	Photos   PhotoStore
	Notifier Notifier
	Now      func() time.Time
}

// maxListed bounds one admin page.
const maxListed = 200

// now reads the service clock.
func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Window says how long after delivery a kind of request may be made; zero means the kind is unknown.
func Window(kind string) time.Duration {
	switch kind {
	case KindExchange:
		return exchangeWindow
	case KindDefect, KindWrongItem:
		return problemWindow
	}
	return 0
}

// Eligible reports whether this order may open a ticket of this kind now.
func Eligible(order Order, kind string, now time.Time) error {
	window := Window(kind)
	switch {
	case window == 0:
		return ErrInvalid
	case order.Status != "delivered" || order.DeliveredAt == nil:
		return ErrNotDelivered
	case now.After(order.DeliveredAt.Add(window)):
		return ErrWindowClosed
	}
	return nil
}

// Create checks the request against the order's delivery date, stores the photos privately and opens a ticket.
// Photos are removed again if the ticket is not saved, so a refused request leaves no files behind.
func (s Service) Create(ctx context.Context, order Order, sub Submission) (Ticket, error) {
	sub.Message = strings.TrimSpace(sub.Message)
	length := utf8.RuneCountInString(sub.Message)
	if length < minMessage || length > maxMessage || len(sub.Photos) > MaxPhotos {
		return Ticket{}, ErrInvalid
	}
	if err := Eligible(order, sub.Kind, s.now()); err != nil {
		return Ticket{}, err
	}
	var names []string
	cleanup := func() {
		for _, name := range names {
			s.Photos.Remove(name)
		}
	}
	for _, photo := range sub.Photos {
		name, err := s.Photos.Save(photo)
		if err != nil {
			cleanup()
			return Ticket{}, err
		}
		names = append(names, name)
	}
	ticket, err := s.Store.Create(ctx, order.ID, sub.Kind, sub.Message, names)
	if err != nil {
		cleanup()
		return Ticket{}, err
	}
	if s.Notifier != nil {
		s.Notifier.ReturnRequested(ctx, ticket)
	}
	return ticket, nil
}

// List returns tickets for staff, open ones first; status "" means every status.
func (s Service) List(ctx context.Context, status string) ([]Ticket, error) {
	if status != "" && status != StatusOpen && status != StatusInProgress && status != StatusClosed {
		return nil, ErrInvalid
	}
	list, err := s.Store.List(ctx, status, maxListed)
	for i := range list {
		s.addDeadline(&list[i])
	}
	return list, err
}

// Get returns one ticket for staff.
func (s Service) Get(ctx context.Context, id int64) (Ticket, error) {
	if id <= 0 {
		return Ticket{}, ErrNotFound
	}
	ticket, err := s.Store.Get(ctx, id)
	s.addDeadline(&ticket)
	return ticket, err
}

// addDeadline fills the refund deadline for problem tickets and flags one that passed while still open.
func (s Service) addDeadline(t *Ticket) {
	if t.Kind == KindExchange || t.CreatedAt.IsZero() {
		return
	}
	due := t.CreatedAt.Add(RefundDeadline)
	t.RefundDue = &due
	t.Overdue = t.Status != StatusClosed && s.now().After(due)
}

// Update moves a ticket along or closes it. Closing needs an outcome other than "refunded" (that is
// RecordRefund's job), and declining needs a note saying why.
func (s Service) Update(ctx context.Context, id int64, status, outcome, note string) error {
	note = strings.TrimSpace(note)
	if id <= 0 {
		return ErrNotFound
	}
	if utf8.RuneCountInString(note) > maxNote {
		return ErrInvalid
	}
	switch status {
	case StatusOpen, StatusInProgress:
		if outcome != "" {
			return ErrInvalid
		}
	case StatusClosed:
		if outcome != OutcomeExchanged && outcome != OutcomeReplaced && outcome != OutcomeDeclined {
			return ErrInvalid
		}
		if outcome == OutcomeDeclined && note == "" {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return s.Store.Update(ctx, id, status, outcome, note)
}

// RecordRefund records money actually sent back and closes the ticket as refunded. Only problem tickets are
// refunded (a change of mind is an exchange), never more than the order total, and never with a future date.
func (s Service) RecordRefund(ctx context.Context, id int64, r Refund, note string) error {
	r.Reference = strings.TrimSpace(r.Reference)
	note = strings.TrimSpace(note)
	if r.PaidAt.IsZero() {
		r.PaidAt = s.now()
	}
	if r.Amount <= 0 || (r.Method != "bkash" && r.Method != "cash") || (r.Method == "bkash" && r.Reference == "") ||
		utf8.RuneCountInString(r.Reference) > maxReference || utf8.RuneCountInString(note) > maxNote || r.PaidAt.After(s.now()) {
		return ErrInvalid
	}
	ticket, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if ticket.Kind == KindExchange || ticket.Status == StatusClosed {
		return ErrNotRefundable
	}
	if r.Amount > ticket.OrderTotal {
		return ErrInvalid
	}
	return s.Store.RecordRefund(ctx, id, r, note)
}

// PhotoPath returns the private file path of one ticket photo for the admin download.
func (s Service) PhotoPath(ctx context.Context, id int64, index int) (string, error) {
	ticket, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if index < 0 || index >= len(ticket.Photos) {
		return "", ErrNotFound
	}
	return s.Photos.Path(ticket.Photos[index])
}
