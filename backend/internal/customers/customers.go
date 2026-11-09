// Package customers owns optional customer accounts: Google or email + password sign-in, the
// name + mobile profile, browser sessions, "My orders" and the phone-checked linking of earlier guest orders.
package customers

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"bongkey/backend/internal/mail"
	"bongkey/backend/internal/security"
)

// SessionTTL is how long a customer stays signed in without signing in again.
// shortcut: one fixed 30-day session with no rotation; add refresh rotation if accounts start holding more than order history.
const SessionTTL = 30 * 24 * time.Hour

// linkTriesPerWindow bounds phone guesses per customer in the limiter's 15-minute window.
const linkTriesPerWindow = 5

// Password sign-in limits per 15-minute window: per email stops guessing one account,
// per client stops one device trying many accounts.
const (
	loginTriesPerEmail  = 5
	loginTriesPerClient = 20
)

var (
	// ErrNotConfigured means GOOGLE_CLIENT_ID is not set, so sign-in is off.
	ErrNotConfigured = errors.New("google sign-in is not configured")
	// ErrSignInFailed hides why a Google token was refused.
	ErrSignInFailed = errors.New("google sign-in failed")
	// ErrNoSession means the cookie is missing, unknown or expired.
	ErrNoSession = errors.New("customer session not found")
	// ErrInvalidPhone means the typed phone cannot be a Bangladeshi mobile number.
	ErrInvalidPhone = errors.New("invalid phone")
	// ErrLinkLimited means too many link attempts in the window.
	ErrLinkLimited = errors.New("too many link attempts")
	// ErrLoginFailed hides whether the email, the password, or the account's lack of a password was wrong.
	ErrLoginFailed = errors.New("wrong email or password")
	// ErrLoginLimited means too many password tries in the window.
	ErrLoginLimited = errors.New("too many sign-in attempts")
	// ErrInvalidProof means the email-code proof is missing, altered or older than ten minutes.
	ErrInvalidProof = errors.New("invalid email proof")
	// ErrInvalidName means the profile name is empty or too long.
	ErrInvalidName = errors.New("invalid name")
	// ErrNotFound means no customer has this email.
	ErrNotFound = errors.New("customer not found")
)

// Customer is the signed-in person. The email is verified (by Google or by an email code);
// the phone is "01XXXXXXXXX" or empty until the profile is finished.
type Customer struct {
	ID    int64  `json:"-"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// Order is one row of "My orders": enough to recognise the order, nothing private.
type Order struct {
	ID          int64     `json:"id"`
	OrderNumber string    `json:"order_number"`
	Total       int       `json:"total"`
	Status      string    `json:"order_status"`
	CreatedAt   time.Time `json:"created_at"`
}

// GoogleVerifier checks a Google ID token; security.GoogleVerifier implements it.
type GoogleVerifier interface {
	Verify(ctx context.Context, raw string) (security.GoogleIdentity, error)
}

// Limiter counts one attempt for a key and returns the attempts in its window.
type Limiter interface {
	Count(ctx context.Context, key string) (int, error)
}

// Store is the customers persistence boundary.
type Store interface {
	SaveGoogleCustomer(ctx context.Context, identity security.GoogleIdentity) (Customer, error)
	// PasswordCustomer returns the customer and password hash for an email (hash "" when only Google is set), or ErrNotFound.
	PasswordCustomer(ctx context.Context, email string) (Customer, string, error)
	// SetPassword creates the customer or replaces its password, and ends all its existing sessions.
	SetPassword(ctx context.Context, email, passwordHash string) (Customer, error)
	UpdateProfile(ctx context.Context, customerID int64, name, phone string) error
	CreateSession(ctx context.Context, customerID int64, tokenHash string, expires time.Time) error
	SessionCustomer(ctx context.Context, tokenHash string) (Customer, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	Orders(ctx context.Context, customerID int64) ([]Order, error)
	LinkCandidates(ctx context.Context, email string) (int, error)
	LinkOrders(ctx context.Context, customerID int64, email, phoneTail string) (int, error)
}

// Service runs customer account rules. It never touches admin sessions.
type Service struct {
	Store   Store
	Google  GoogleVerifier
	Limiter Limiter
	// Secret keys the limiter's HMAC so stored keys do not reveal customer IDs.
	Secret string
}

// SignIn verifies the Google token, saves the customer by Google ID and starts a new session.
// It returns the raw session token for the cookie; only its hash is stored.
func (s Service) SignIn(ctx context.Context, credential string) (Customer, string, error) {
	if s.Google == nil {
		return Customer{}, "", ErrNotConfigured
	}
	identity, err := s.Google.Verify(ctx, credential)
	if err != nil {
		return Customer{}, "", ErrSignInFailed
	}
	customer, err := s.Store.SaveGoogleCustomer(ctx, identity)
	if err != nil {
		return Customer{}, "", err
	}
	return s.startSession(ctx, customer)
}

// Login checks an email and password and starts a session. Unknown emails, Google-only accounts and
// wrong passwords all give ErrLoginFailed after the same bcrypt work, so the answer reveals nothing.
func (s Service) Login(ctx context.Context, rawEmail, password, clientIP string) (Customer, string, error) {
	email, ok := normalizeEmail(rawEmail)
	if !ok || password == "" {
		return Customer{}, "", ErrLoginFailed
	}
	if s.Limiter == nil || clientIP == "" {
		return Customer{}, "", errors.New("customer login limiter is not configured")
	}
	limits := []struct {
		key string
		max int
	}{
		{security.EmailKey(s.Secret, "customer-login-email", email), loginTriesPerEmail},
		{security.EmailKey(s.Secret, "customer-login-ip", clientIP), loginTriesPerClient},
	}
	for _, limit := range limits {
		tries, err := s.Limiter.Count(ctx, limit.key)
		if err != nil {
			return Customer{}, "", err
		}
		if tries > limit.max {
			return Customer{}, "", ErrLoginLimited
		}
	}
	customer, hash, err := s.Store.PasswordCustomer(ctx, email)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Customer{}, "", err
	}
	if !security.CheckPassword(hash, password) {
		return Customer{}, "", ErrLoginFailed
	}
	return s.startSession(ctx, customer)
}

// SetPassword is both "create account" and "forgot password": the email-code proof shows the person owns
// the email, so it creates the account or replaces the password (also on a Google account with that email).
// Every older session ends, so a reset locks out anyone who knew the old password.
func (s Service) SetPassword(ctx context.Context, emailProof, password string) (Customer, string, error) {
	email, err := security.VerifyEmailProof(s.Secret, emailProof)
	if err != nil {
		return Customer{}, "", ErrInvalidProof
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return Customer{}, "", err
	}
	customer, err := s.Store.SetPassword(ctx, email, hash)
	if err != nil {
		return Customer{}, "", err
	}
	return s.startSession(ctx, customer)
}

// UpdateProfile saves the required name and Bangladeshi mobile number.
// shortcut: the phone is typed, not SMS-verified; add an SMS code when an SMS provider is chosen.
func (s Service) UpdateProfile(ctx context.Context, customer Customer, name, phone string) (Customer, error) {
	name = strings.Join(strings.Fields(name), " ")
	if n := utf8.RuneCountInString(name); n < 2 || n > 80 {
		return Customer{}, ErrInvalidName
	}
	tail, ok := phoneTail(phone)
	if !ok {
		return Customer{}, ErrInvalidPhone
	}
	customer.Name, customer.Phone = name, "0"+tail
	return customer, s.Store.UpdateProfile(ctx, customer.ID, customer.Name, customer.Phone)
}

// startSession creates a new random session for the customer and returns the raw token for the cookie.
func (s Service) startSession(ctx context.Context, customer Customer) (Customer, string, error) {
	token, hash, err := security.NewSessionToken()
	if err != nil {
		return Customer{}, "", err
	}
	if err := s.Store.CreateSession(ctx, customer.ID, hash, time.Now().Add(SessionTTL)); err != nil {
		return Customer{}, "", err
	}
	return customer, token, nil
}

// normalizeEmail lower-cases one ordinary mailbox, matching how email codes and checkout store it.
func normalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	return email, mail.ValidAddress(email)
}

// Authenticate returns the customer behind a session token, or ErrNoSession.
func (s Service) Authenticate(ctx context.Context, token string) (Customer, error) {
	if token == "" || len(token) > 128 {
		return Customer{}, ErrNoSession
	}
	return s.Store.SessionCustomer(ctx, security.SessionTokenHash(token))
}

// Logout deletes the session so the cookie stops working everywhere at once.
func (s Service) Logout(ctx context.Context, token string) error {
	if token == "" || len(token) > 128 {
		return nil
	}
	return s.Store.DeleteSession(ctx, security.SessionTokenHash(token))
}

// Orders lists the customer's linked orders, newest first.
func (s Service) Orders(ctx context.Context, customer Customer) ([]Order, error) {
	return s.Store.Orders(ctx, customer.ID)
}

// LinkCandidates counts unlinked orders that carry the customer's Google-verified email.
// Showing the count is safe: the email is proven theirs, and linking still needs the phone.
func (s Service) LinkCandidates(ctx context.Context, customer Customer) (int, error) {
	return s.Store.LinkCandidates(ctx, customer.Email)
}

// Link attaches unlinked orders whose email matches the verified email and whose phone matches the typed one.
// An email match alone is never enough, because anyone can type someone else's email at checkout.
// It returns how many orders were newly linked; a wrong phone simply links 0, so nothing is revealed.
func (s Service) Link(ctx context.Context, customer Customer, phone string) (int, error) {
	tail, ok := phoneTail(phone)
	if !ok {
		return 0, ErrInvalidPhone
	}
	if s.Limiter == nil {
		return 0, errors.New("customer link limiter is not configured")
	}
	tries, err := s.Limiter.Count(ctx, security.EmailKey(s.Secret, "customer-link", strconv.FormatInt(customer.ID, 10)))
	if err != nil {
		return 0, err
	}
	if tries > linkTriesPerWindow {
		return 0, ErrLinkLimited
	}
	return s.Store.LinkOrders(ctx, customer.ID, customer.Email, tail)
}

// phoneTail keeps the last ten digits of a Bangladeshi mobile ("1XXXXXXXXX"), so "017…", "+88017…" and "88017…" all match.
func phoneTail(phone string) (string, bool) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	switch {
	case len(digits) == 13 && strings.HasPrefix(digits, "880"):
		digits = digits[3:]
	case len(digits) == 11 && digits[0] == '0':
		digits = digits[1:]
	}
	if len(digits) != 10 || digits[0] != '1' || digits[1] < '3' {
		return "", false
	}
	return digits, true
}
