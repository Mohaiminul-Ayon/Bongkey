// Package callbacks records a shopper's request to be called about a custom lens, so staff can talk the
// lens through by phone or chat before any order exists.
package callbacks

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"bongkey/backend/internal/orders"
	"bongkey/backend/internal/security"
)

var (
	// ErrInvalid means a field is missing or malformed.
	ErrInvalid = errors.New("invalid call-back request")
	// ErrLimited means this phone or client asked too often in the current window.
	ErrLimited = errors.New("too many call-back requests")
	// ErrNotFound means no request has that ID.
	ErrNotFound = errors.New("call-back request not found")
)

// Request statuses: open until staff have called, then done.
const (
	StatusOpen = "open"
	StatusDone = "done"
)

// Field limits match the database checks so a bad value is a clear 400, not a constraint error.
const (
	minName, maxName       = 2, 80
	minMessage, maxMessage = 5, 500
)

// Requests per 15-minute limiter window: a phone gets a few retries, a client a few phones (a shared shop Wi-Fi).
const (
	requestsPerPhone  = 3
	requestsPerClient = 6
)

var slugPattern = regexp.MustCompile(`^[a-z0-9-]{1,160}$`)

// Request is one saved call-back request as the owner sees it.
type Request struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Phone       string     `json:"phone"`
	ProductSlug string     `json:"product_slug"`
	Message     string     `json:"message"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	HandledAt   *time.Time `json:"handled_at"`
}

// Store persists requests.
type Store interface {
	Create(ctx context.Context, r Request) (Request, error)
	List(ctx context.Context, status string, limit int) ([]Request, error)
	MarkDone(ctx context.Context, id int64) error
}

// Notifier queues the owner's alert; notifications.Service implements it and records its own errors.
type Notifier interface {
	CallbackRequested(ctx context.Context, r Request)
}

// Service validates, rate-limits and saves requests. Notifier may be nil (no owner email configured).
type Service struct {
	Store    Store
	Limiter  orders.AttemptCounter
	Notifier Notifier
	// Secret keys the HMACs that hide the phone and client IP in limiter keys.
	Secret string
}

// maxListed bounds one admin page; older requests stay in the table.
const maxListed = 200

// Create checks and saves a new request, then alerts the owner. The phone and IP are counted before
// validation, so a flood of malformed requests is limited too.
func (s Service) Create(ctx context.Context, r Request, clientIP string) (Request, error) {
	if s.Limiter == nil || clientIP == "" {
		return Request{}, errors.New("call-back limiter is not configured")
	}
	r.Name = strings.Join(strings.Fields(r.Name), " ")
	r.Phone = orders.NormalizeMobile(r.Phone)
	r.ProductSlug = strings.TrimSpace(r.ProductSlug)
	r.Message = strings.TrimSpace(r.Message)
	clientCount, err := s.Limiter.Count(ctx, security.EmailKey(s.Secret, "callback-ip", clientIP))
	if err != nil {
		return Request{}, err
	}
	if clientCount > requestsPerClient {
		return Request{}, ErrLimited
	}
	if !valid(r) {
		return Request{}, ErrInvalid
	}
	phoneCount, err := s.Limiter.Count(ctx, security.EmailKey(s.Secret, "callback-phone", r.Phone))
	if err != nil {
		return Request{}, err
	}
	if phoneCount > requestsPerPhone {
		return Request{}, ErrLimited
	}
	saved, err := s.Store.Create(ctx, r)
	if err != nil {
		return Request{}, err
	}
	if s.Notifier != nil {
		s.Notifier.CallbackRequested(ctx, saved)
	}
	return saved, nil
}

// valid checks the normalized fields against the same limits as the database.
func valid(r Request) bool {
	name, message := utf8.RuneCountInString(r.Name), utf8.RuneCountInString(r.Message)
	return name >= minName && name <= maxName && orders.ValidMobile(r.Phone) &&
		message >= minMessage && message <= maxMessage && (r.ProductSlug == "" || slugPattern.MatchString(r.ProductSlug))
}

// List returns the newest requests for the owner; status "" means every status.
func (s Service) List(ctx context.Context, status string) ([]Request, error) {
	if status != "" && status != StatusOpen && status != StatusDone {
		return nil, ErrInvalid
	}
	return s.Store.List(ctx, status, maxListed)
}

// MarkDone records that staff have called; marking a done request again changes nothing.
func (s Service) MarkDone(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrNotFound
	}
	return s.Store.MarkDone(ctx, id)
}
