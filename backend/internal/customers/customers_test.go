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

// memoryStore keeps sessions in a map, one password account, and records link and profile calls.
type memoryStore struct {
	sessions map[string]time.Time
	linked   []string
	// passwords maps email to bcrypt hash; profile is the last saved "name|phone".
	passwords map[string]string
	profile   string
}

func (m *memoryStore) PasswordCustomer(_ context.Context, email string) (Customer, string, error) {
	hash, ok := m.passwords[email]
	if !ok {
		return Customer{}, "", ErrNotFound
	}
	return Customer{ID: 9, Email: email}, hash, nil
}
func (m *memoryStore) SetPassword(_ context.Context, email, hash string) (Customer, error) {
	m.passwords[email] = hash
	clear(m.sessions)
	return Customer{ID: 9, Email: email}, nil
}
func (m *memoryStore) UpdateProfile(_ context.Context, _ int64, name, phone string) error {
	m.profile = name + "|" + phone
	return nil
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

const testSecret = "synthetic-test-secret-at-least-thirty-two-characters"

// TestSetPasswordAndLogin covers sign-up by email proof, sign-in, wrong passwords, unknown emails and the limit.
func TestSetPasswordAndLogin(t *testing.T) {
	ctx := context.Background()
	store := &memoryStore{sessions: map[string]time.Time{}, passwords: map[string]string{}}
	service := Service{Store: store, Limiter: &counter{}, Secret: testSecret}
	proof, err := security.IssueEmailProof(testSecret, "new@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.SetPassword(ctx, "forged-proof", "long enough"); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("forged proof: %v", err)
	}
	if _, _, err := service.SetPassword(ctx, proof, "short"); !errors.Is(err, security.ErrWeakPassword) {
		t.Fatalf("short password: %v", err)
	}
	customer, token, err := service.SetPassword(ctx, proof, "long enough")
	if err != nil || customer.Email != "new@example.test" || len(token) != 64 {
		t.Fatalf("sign-up: %+v %v", customer, err)
	}
	if store.passwords["new@example.test"] == "long enough" {
		t.Fatal("plain password stored")
	}
	if _, _, err := service.Login(ctx, " NEW@example.test ", "long enough", "203.0.113.5"); err != nil {
		t.Fatalf("right password refused: %v", err)
	}
	for name, input := range map[string][2]string{
		"wrong password": {"new@example.test", "long enougH"},
		"unknown email":  {"nobody@example.test", "long enough"},
		"bad email":      {"not-an-email", "long enough"},
	} {
		if _, _, err := service.Login(ctx, input[0], input[1], "203.0.113.5"); !errors.Is(err, ErrLoginFailed) {
			t.Errorf("%s: %v", name, err)
		}
	}
	limited := Service{Store: store, Limiter: &counter{n: loginTriesPerEmail}, Secret: testSecret}
	if _, _, err := limited.Login(ctx, "new@example.test", "long enough", "203.0.113.5"); !errors.Is(err, ErrLoginLimited) {
		t.Fatalf("over the limit: %v", err)
	}
}

// TestUpdateProfile checks the name and phone rules and the saved, normalized values.
func TestUpdateProfile(t *testing.T) {
	store := &memoryStore{}
	service := Service{Store: store}
	customer := Customer{ID: 9, Email: "new@example.test"}
	for name, input := range map[string][2]string{
		"empty name": {" ", "01712345678"},
		"bad phone":  {"Rahim Uddin", "12345"},
	} {
		if _, err := service.UpdateProfile(context.Background(), customer, input[0], input[1]); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	got, err := service.UpdateProfile(context.Background(), customer, "  Rahim   Uddin ", "+880 1712-345678")
	if err != nil || got.Name != "Rahim Uddin" || got.Phone != "01712345678" || store.profile != "Rahim Uddin|01712345678" {
		t.Fatalf("profile: %+v %q %v", got, store.profile, err)
	}
}
