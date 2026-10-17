package auth

import (
	"context"
	"database/sql"
	"errors"

	"bongkey/backend/internal/security"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials hides whether the email or password was wrong.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrInvalidSession hides whether a token or its admin account was invalid.
var ErrInvalidSession = errors.New("invalid admin session")

// Admin is the persisted identity used by the owner-only admin area.
type Admin struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
	Role         string
}

// AdminRepository supplies identities without exposing SQL to business rules.
type AdminRepository interface {
	ByEmail(context.Context, string) (Admin, error)
	ByID(context.Context, int64) (Admin, error)
}

// PostgresRepository reads current admin identity and role from PostgreSQL.
type PostgresRepository struct{ DB *sql.DB }

// ByEmail loads the account needed for a password login.
func (r PostgresRepository) ByEmail(ctx context.Context, email string) (Admin, error) {
	var admin Admin
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, email, password_hash, role FROM admins WHERE email = $1`, email).
		Scan(&admin.ID, &admin.Name, &admin.Email, &admin.PasswordHash, &admin.Role)
	return admin, err
}

// ByID checks the current account and role on every protected request.
func (r PostgresRepository) ByID(ctx context.Context, id int64) (Admin, error) {
	var admin Admin
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, email, role FROM admins WHERE id = $1`, id).
		Scan(&admin.ID, &admin.Name, &admin.Email, &admin.Role)
	return admin, err
}

// Service owns password and current-role checks for admin sessions.
type Service struct {
	Repository AdminRepository
	JWTSecret  string
}

// Login checks the stored password and owner role before issuing a session.
func (s Service) Login(ctx context.Context, email, password string) (Admin, string, error) {
	admin, err := s.Repository.ByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return Admin{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return Admin{}, "", err
	}
	if admin.Role != "owner" || bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return Admin{}, "", ErrInvalidCredentials
	}
	token, err := security.IssueAdminToken(s.JWTSecret, admin.ID)
	if err != nil {
		return Admin{}, "", err
	}
	return admin, token, nil
}

// Authenticate requires a valid token and a currently existing owner account.
func (s Service) Authenticate(ctx context.Context, token string) (Admin, error) {
	id, err := security.VerifyAdminToken(s.JWTSecret, token)
	if err != nil {
		return Admin{}, ErrInvalidSession
	}
	admin, err := s.Repository.ByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Admin{}, ErrInvalidSession
	}
	if err != nil {
		return Admin{}, err
	}
	if admin.ID != id || admin.Role != "owner" {
		return Admin{}, ErrInvalidSession
	}
	return admin, nil
}
