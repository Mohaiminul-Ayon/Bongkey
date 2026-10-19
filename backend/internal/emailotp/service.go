package emailotp

import (
	"context"
	"errors"
	"strings"

	"bongkey/backend/internal/security"
)

var (
	// ErrInvalid hides whether a code, email, or expired challenge caused rejection.
	ErrInvalid = errors.New("invalid email verification")
	// ErrLimited means the email or connection must wait before requesting another code.
	ErrLimited = errors.New("email verification rate limited")
)

// Store persists OTP attempts and rate limits across backend instances.
type Store interface {
	Reserve(context.Context, string, string, string) error
	Release(context.Context, string, string) error
	Consume(context.Context, string, string) (bool, error)
}

// Sender delivers one code without exposing it to HTTP responses or logs.
type Sender interface {
	Send(context.Context, string, string) error
}

// Service coordinates one-time email verification without creating an account.
type Service struct {
	Store  Store
	Sender Sender
	Secret string
}

// Request reserves a bounded challenge before sending one code to the mailbox.
func (s Service) Request(ctx context.Context, rawEmail, remoteIP string) error {
	email, ok := normalizeEmail(rawEmail)
	if !ok || remoteIP == "" || len(s.Secret) < 32 {
		return ErrInvalid
	}
	code, err := security.NewEmailCode()
	if err != nil {
		return err
	}
	emailKey := security.EmailKey(s.Secret, "email", email)
	codeHash := security.EmailCodeHash(s.Secret, email, code)
	ipKey := security.EmailKey(s.Secret, "ip", remoteIP)
	if err := s.Store.Reserve(ctx, emailKey, ipKey, codeHash); err != nil {
		return err
	}
	if err := s.Sender.Send(ctx, email, code); err != nil {
		if releaseErr := s.Store.Release(ctx, emailKey, codeHash); releaseErr != nil {
			return errors.Join(err, releaseErr)
		}
		return err
	}
	return nil
}

// Verify consumes the matching code once and returns a short-lived email proof.
func (s Service) Verify(ctx context.Context, rawEmail, code string) (string, error) {
	email, ok := normalizeEmail(rawEmail)
	if !ok || len(code) != 6 || len(s.Secret) < 32 {
		return "", ErrInvalid
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			return "", ErrInvalid
		}
	}
	matched, err := s.Store.Consume(ctx, security.EmailKey(s.Secret, "email", email), security.EmailCodeHash(s.Secret, email, code))
	if err != nil {
		return "", err
	}
	if !matched {
		return "", ErrInvalid
	}
	return security.IssueEmailProof(s.Secret, email)
}

// normalizeEmail accepts one ordinary mailbox and uses the same identity at checkout.
func normalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	return email, validAddress(email)
}
