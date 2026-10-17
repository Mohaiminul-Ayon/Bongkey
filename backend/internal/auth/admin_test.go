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

// TestAdminSessionRequiresCurrentOwner checks login and post-login role changes.
func TestAdminSessionRequiresCurrentOwner(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("long-test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	admin := Admin{ID: 3, Name: "Owner", Email: "owner@example.test", PasswordHash: string(hash), Role: "owner"}
	service := Service{Repository: fakeRepository{admin: admin}, JWTSecret: testSecret}
	_, token, err := service.Login(context.Background(), admin.Email, "long-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("owner session rejected: %v", err)
	}
	if _, _, err := service.Login(context.Background(), admin.Email, "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	service.Repository = fakeRepository{admin: Admin{ID: 3, PasswordHash: string(hash), Role: "customer"}}
	if _, _, err := service.Login(context.Background(), admin.Email, "long-test-password"); !errors.Is(err, ErrInvalidCredentials) {
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
