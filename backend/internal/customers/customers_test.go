package customers

import (
	"context"
	"errors"
	"testing"
	"time"

	"bongkey/backend/internal/security"
)

// fakeGoogle accepts one credential string.
type fakeGoogle struct{}

// Verify returns a fixed identity only for the "good" credential.
func (fakeGoogle) Verify(_ context.Context, raw string) (security.GoogleIdentity, error) {
	if raw != "good" {
		return security.GoogleIdentity{}, security.ErrInvalidGoogleToken
	}
	return security.GoogleIdentity{Subject: "sub-1", Email: "rahim@example.test", Name: "Rahim"}, nil
}

// memoryStore keeps sessions in a map and records link calls.
type memoryStore struct {
	sessions map[string]time.Time
	linked   []string
}

func (m *memoryStore) SaveGoogleCustomer(_ context.Context, id security.GoogleIdentity) (Customer, error) {
	return Customer{ID: 7, Email: id.Email, Name: id.Name}, nil
}
func (m *memoryStore) CreateSession(_ context.Context, _ int64, hash string, expires time.Time) error {
	m.sessions[hash] = expires
	return nil
}
func (m *memoryStore) SessionCustomer(_ context.Context, hash string) (Customer, error) {
	if expires, ok := m.sessions[hash]; ok && time.Now().Before(expires) {
		return Customer{ID: 7, Email: "rahim@example.test"}, nil
	}
	return Customer{}, ErrNoSession
}
func (m *memoryStore) DeleteSession(_ context.Context, hash string) error {
	delete(m.sessions, hash)
	return nil
}
func (m *memoryStore) Orders(context.Context, int64) ([]Order, error)      { return nil, nil }
func (m *memoryStore) LinkCandidates(context.Context, string) (int, error) { return 0, nil }
func (m *memoryStore) LinkOrders(_ context.Context, _ int64, email, tail string) (int, error) {
	m.linked = append(m.linked, email+"|"+tail)
	return 1, nil
}

// counter is a limiter that counts every key together.
type counter struct{ n int }

func (c *counter) Count(context.Context, string) (int, error) { c.n++; return c.n, nil }

// TestSessionLifecycle stores only the token hash, and logout or expiry ends access at once.
func TestSessionLifecycle(t *testing.T) {
	store := &memoryStore{sessions: map[string]time.Time{}}
	service := Service{Store: store, Google: fakeGoogle{}}
	if _, _, err := service.SignIn(context.Background(), "forged"); !errors.Is(err, ErrSignInFailed) {
		t.Fatalf("forged credential: %v", err)
	}
	if _, _, err := (Service{Store: store}).SignIn(context.Background(), "good"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("sign-in without client ID: %v", err)
	}
	customer, token, err := service.SignIn(context.Background(), "good")
	if err != nil || customer.Email != "rahim@example.test" || len(token) != 64 {
		t.Fatalf("sign-in: %+v %q %v", customer, token, err)
	}
	if _, raw := store.sessions[token]; raw {
		t.Fatal("raw session token stored")
	}
	if _, err := service.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("fresh session refused: %v", err)
	}
	if err := service.Logout(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("session works after logout: %v", err)
	}

	_, token, _ = service.SignIn(context.Background(), "good")
	store.sessions[security.SessionTokenHash(token)] = time.Now().Add(-time.Second)
	if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("expired session accepted: %v", err)
	}
}

// TestLinkRules checks the phone format and the attempt limit before any order is touched.
func TestLinkRules(t *testing.T) {
	store := &memoryStore{}
	limiter := &counter{}
	service := Service{Store: store, Limiter: limiter, Secret: "synthetic-test-secret-at-least-thirty-two-characters"}
	customer := Customer{ID: 7, Email: "rahim@example.test"}
	for _, phone := range []string{"", "12345", "01212345678", "+1 555 123 4567"} {
		if _, err := service.Link(context.Background(), customer, phone); !errors.Is(err, ErrInvalidPhone) {
			t.Errorf("phone %q: %v", phone, err)
		}
	}
	for _, phone := range []string{"01712-345678", "+880 1712 345678", "8801712345678"} {
		if _, err := service.Link(context.Background(), customer, phone); err != nil {
			t.Fatalf("phone %q: %v", phone, err)
		}
	}
	for _, got := range store.linked {
		if got != "rahim@example.test|1712345678" {
			t.Fatalf("link used %q", got)
		}
	}
	for range linkTriesPerWindow - 3 {
		_, _ = service.Link(context.Background(), customer, "01712345678")
	}
	if _, err := service.Link(context.Background(), customer, "01712345678"); !errors.Is(err, ErrLinkLimited) {
		t.Fatalf("sixth try: %v", err)
	}
	if len(store.linked) != linkTriesPerWindow {
		t.Fatalf("store reached %d times, want %d", len(store.linked), linkTriesPerWindow)
	}
}
