package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

const testSecret = "synthetic-test-secret-at-least-thirty-two-characters"

type fakeRepository struct {
	admin Admin
	err   error
}

// ByEmail supplies the fake account for login tests.
func (r fakeRepository) ByEmail(context.Context, string) (Admin, error) { return r.admin, r.err }

// ByID supplies the current fake account for session checks.
func (r fakeRepository) ByID(context.Context, int64) (Admin, error) { return r.admin, r.err }

// countingLimiter records attempts per client key like the shared PostgreSQL window.
type countingLimiter struct{ attempts map[string]int }

// Count increments and returns the in-memory attempt count for one client key.
func (l *countingLimiter) Count(_ context.Context, key string) (int, error) {
	if l.attempts == nil {
		l.attempts = map[string]int{}
	}
	l.attempts[key]++
	return l.attempts[key], nil
}

// TestLoginRateLimitStopsPasswordGuessing blocks one client after the budget while another client still works.
func TestLoginRateLimitStopsPasswordGuessing(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("long-test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	service := Service{Repository: fakeRepository{admin: Admin{ID: 3, PasswordHash: string(hash), Role: "owner"}}, Limiter: &countingLimiter{}, JWTSecret: testSecret}
	for i := 0; i < loginAttemptsPerWindow; i++ {
		if _, _, err := service.Login(context.Background(), "owner@example.test", "wrong", "192.0.2.1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	if _, _, err := service.Login(context.Background(), "owner@example.test", "long-test-password", "192.0.2.1"); !errors.Is(err, ErrLoginLimited) {
		t.Fatalf("limited client reached password check: %v", err)
	}
	if _, _, err := service.Login(context.Background(), "owner@example.test", "long-test-password", "192.0.2.2"); err != nil {
		t.Fatalf("other client was blocked: %v", err)
	}
	if _, _, err := (Service{Repository: fakeRepository{admin: Admin{ID: 3}}, JWTSecret: testSecret}).Login(context.Background(), "owner@example.test", "x", "192.0.2.3"); err == nil {
		t.Fatal("login without a limiter must fail closed")
	}
}

// TestAdminSessionRequiresCurrentOwner checks login and post-login role changes.
func TestAdminSessionRequiresCurrentOwner(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("long-test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	admin := Admin{ID: 3, Name: "Owner", Email: "owner@example.test", PasswordHash: string(hash), Role: "owner"}
	service := Service{Repository: fakeRepository{admin: admin}, Limiter: &countingLimiter{}, JWTSecret: testSecret}
	_, token, err := service.Login(context.Background(), admin.Email, "long-test-password", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("owner session rejected: %v", err)
	}
	if _, _, err := service.Login(context.Background(), admin.Email, "wrong", "192.0.2.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	service.Repository = fakeRepository{admin: Admin{ID: 3, PasswordHash: string(hash), Role: "customer"}}
	if _, _, err := service.Login(context.Background(), admin.Email, "long-test-password", "192.0.2.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong role login: %v", err)
	}
	service.Repository = fakeRepository{admin: Admin{ID: 3, Role: "customer"}}
	if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("changed role retained access: %v", err)
	}
	service.Repository = fakeRepository{admin: Admin{ID: 9, Role: "owner"}}
	if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("wrong account retained access: %v", err)
	}
	service.Repository = fakeRepository{err: sql.ErrNoRows}
	if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("deleted admin retained access: %v", err)
	}
}
