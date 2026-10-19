package emailotp

import (
	"context"
	"errors"
	"testing"

	"bongkey/backend/internal/security"
)

type codeStore struct {
	emailKey, codeHash string
	used, released     bool
}

// Reserve remembers only scoped hashes for the service test.
func (s *codeStore) Reserve(_ context.Context, emailKey, _, codeHash string) error {
	s.emailKey, s.codeHash = emailKey, codeHash
	return nil
}

// Release invalidates a code when sending fails.
func (s *codeStore) Release(_ context.Context, emailKey, codeHash string) error {
	if emailKey == s.emailKey && codeHash == s.codeHash {
		s.released = true
	}
	return nil
}

// Consume accepts only the saved hash and rejects reuse.
func (s *codeStore) Consume(_ context.Context, emailKey, codeHash string) (bool, error) {
	if s.used || s.released || emailKey != s.emailKey || codeHash != s.codeHash {
		return false, nil
	}
	s.used = true
	return true, nil
}

type codeSender struct {
	email, code string
	err         error
}

// Send captures the recipient and code without using a real mailbox.
func (s *codeSender) Send(_ context.Context, email, code string) error {
	s.email, s.code = email, code
	return s.err
}

// TestEmailOTPFlow checks mailbox binding, one-time use, and failed delivery.
func TestEmailOTPFlow(t *testing.T) {
	const secret = "synthetic-email-proof-secret-at-least-32-characters"
	store, sender := &codeStore{}, &codeSender{}
	service := Service{Store: store, Sender: sender, Secret: secret}
	if err := service.Request(context.Background(), " Buyer@Example.com ", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if sender.email != "buyer@example.com" || len(sender.code) != 6 || store.codeHash == sender.code {
		t.Fatal("code was sent to the wrong address or stored in plaintext")
	}
	if store.emailKey != security.EmailKey(secret, "email", "buyer@example.com") {
		t.Fatal("mailbox was not scoped to its hash")
	}
	if _, err := service.Verify(context.Background(), "other@example.com", sender.code); !errors.Is(err, ErrInvalid) {
		t.Fatal("another mailbox used the code")
	}
	wrongCode := "000000"
	if sender.code == wrongCode {
		wrongCode = "000001"
	}
	if _, err := service.Verify(context.Background(), sender.email, wrongCode); !errors.Is(err, ErrInvalid) {
		t.Fatal("wrong code was accepted")
	}
	proof, err := service.Verify(context.Background(), sender.email, sender.code)
	if err != nil {
		t.Fatal(err)
	}
	if email, err := security.VerifyEmailProof(secret, proof); err != nil || email != sender.email {
		t.Fatal("email proof did not bind to the verified mailbox")
	}
	if _, err := service.Verify(context.Background(), sender.email, sender.code); !errors.Is(err, ErrInvalid) {
		t.Fatal("code was reusable")
	}

	failedStore, failedSender := &codeStore{}, &codeSender{err: errors.New("synthetic send failure")}
	failed := Service{Store: failedStore, Sender: failedSender, Secret: secret}
	if err := failed.Request(context.Background(), "buyer@example.com", "127.0.0.1"); err == nil || !failedStore.released {
		t.Fatal("undelivered code remained usable")
	}
}
