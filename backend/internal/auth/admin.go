package auth

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"bongkey/backend/internal/security"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials hides whether the email or password was wrong.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrLoginLimited means this client made too many login attempts and must wait.
var ErrLoginLimited = errors.New("too many admin login attempts")

// loginAttemptsPerWindow bounds password guessing per client IP across all backend instances.
const loginAttemptsPerWindow = 10

// dummyPasswordHash lets unknown emails cost the same bcrypt work, so timing does not reveal the owner email.
var dummyPasswordHash = sync.OnceValue(func() []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte("bongkey-unknown-admin"), bcrypt.DefaultCost)
	return hash
})

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

// LoginLimiter counts one login attempt for a hashed client key and returns the attempts in its window.
type LoginLimiter interface {
	Count(context.Context, string) (int, error)
}

// PostgresLoginLimiter keeps login attempt windows in PostgreSQL so every instance shares one budget.
type PostgresLoginLimiter struct{ DB *sql.DB }

// Count increments the client's 15-minute window atomically and prunes a bounded batch of stale rows.
func (l PostgresLoginLimiter) Count(ctx context.Context, clientKey string) (int, error) {
	if _, err := l.DB.ExecContext(ctx, `DELETE FROM admin_login_limits WHERE client_key IN (SELECT client_key FROM admin_login_limits WHERE window_start < NOW() - INTERVAL '1 day' LIMIT 100)`); err != nil {
		return 0, err
	}
	var attempts int
	err := l.DB.QueryRowContext(ctx, `
		INSERT INTO admin_login_limits (client_key, window_start, attempt_count)
		VALUES ($1, NOW(), 1)
		ON CONFLICT (client_key) DO UPDATE SET
			attempt_count = CASE WHEN admin_login_limits.window_start < NOW() - INTERVAL '15 minutes' THEN 1 ELSE admin_login_limits.attempt_count + 1 END,
			window_start = CASE WHEN admin_login_limits.window_start < NOW() - INTERVAL '15 minutes' THEN NOW() ELSE admin_login_limits.window_start END
		RETURNING attempt_count
	`, clientKey).Scan(&attempts)
	return attempts, err
}

// Service owns password and current-role checks for admin sessions.
type Service struct {
	Repository AdminRepository
	Limiter    LoginLimiter
	JWTSecret  string
}

// Login rate-limits the client, then checks the stored password and owner role before issuing a session.
func (s Service) Login(ctx context.Context, email, password, remoteIP string) (Admin, string, error) {
	if s.Limiter == nil || remoteIP == "" {
		return Admin{}, "", errors.New("admin login limiter is not configured")
	}
	attempts, err := s.Limiter.Count(ctx, security.EmailKey(s.JWTSecret, "admin-login-ip", remoteIP))
	if err != nil {
		return Admin{}, "", err
	}
	if attempts > loginAttemptsPerWindow {
		return Admin{}, "", ErrLoginLimited
	}
	admin, err := s.Repository.ByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))
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
