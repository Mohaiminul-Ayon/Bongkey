package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bongkey/backend/internal/customers"
	"bongkey/backend/internal/middleware"
	"bongkey/backend/internal/security"
	"github.com/gin-gonic/gin"
)

// oneGoogle accepts only the credential "good".
type oneGoogle struct{}

// Verify returns a fixed identity for "good".
func (oneGoogle) Verify(_ context.Context, raw string) (security.GoogleIdentity, error) {
	if raw != "good" {
		return security.GoogleIdentity{}, security.ErrInvalidGoogleToken
	}
	return security.GoogleIdentity{Subject: "s", Email: "rahim@example.test", Name: "Rahim"}, nil
}

// sessionStore keeps session hashes in memory; order methods return nothing.
type sessionStore struct{ hashes map[string]bool }

func (s sessionStore) SaveGoogleCustomer(_ context.Context, id security.GoogleIdentity) (customers.Customer, error) {
	return customers.Customer{ID: 1, Email: id.Email, Name: id.Name}, nil
}
func (s sessionStore) CreateSession(_ context.Context, _ int64, hash string, _ time.Time) error {
	s.hashes[hash] = true
	return nil
}
func (s sessionStore) SessionCustomer(_ context.Context, hash string) (customers.Customer, error) {
	if !s.hashes[hash] {
		return customers.Customer{}, customers.ErrNoSession
	}
	return customers.Customer{ID: 1, Email: "rahim@example.test", Name: "Rahim"}, nil
}
func (s sessionStore) DeleteSession(_ context.Context, hash string) error {
	delete(s.hashes, hash)
	return nil
}
func (s sessionStore) Orders(context.Context, int64) ([]customers.Order, error) {
	return []customers.Order{}, nil
}
func (s sessionStore) LinkCandidates(context.Context, string) (int, error) { return 0, nil }
func (s sessionStore) PasswordCustomer(context.Context, string) (customers.Customer, string, error) {
	return customers.Customer{}, "", customers.ErrNotFound
}
func (s sessionStore) SetPassword(_ context.Context, email, _ string) (customers.Customer, error) {
	return customers.Customer{ID: 1, Email: email}, nil
}
func (s sessionStore) UpdateProfile(context.Context, int64, string, string) error { return nil }
func (s sessionStore) LinkOrders(context.Context, int64, string, string) (int, error) {
	return 0, nil
}

// TestCustomerSessionHTTP walks sign-in, a signed-in read, logout and the CSRF header rule through real routes.
func TestCustomerSessionHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "synthetic-test-secret-at-least-thirty-two-characters"
	service := customers.Service{Store: sessionStore{hashes: map[string]bool{}}, Google: oneGoogle{}, Limiter: &countAll{}, Secret: secret}
	h := CustomerHandler{Service: service, SecureCookie: true}
	router := gin.New()
	group := router.Group("/api/customer", middleware.CustomerCSRF())
	group.POST("/google", h.SignIn)
	group.POST("/logout", h.Logout)
	group.GET("/me", middleware.CustomerAuth(service), h.Me)
	group.POST("/login", h.Login)
	group.POST("/password", h.SetPassword)
	group.PUT("/profile", middleware.CustomerAuth(service), h.UpdateProfile)

	send := func(method, path, body, cookie string, header bool) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		if header {
			request.Header.Set("X-Requested-With", "bongkey")
		}
		if cookie != "" {
			request.AddCookie(&http.Cookie{Name: middleware.CustomerCookie, Value: cookie})
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	if got := send(http.MethodPost, "/api/customer/google", `{"credential":"good"}`, "", false); got.Code != http.StatusForbidden {
		t.Fatalf("sign-in without the CSRF header: %d", got.Code)
	}
	if got := send(http.MethodPost, "/api/customer/google", `{"credential":"forged"}`, "", true); got.Code != http.StatusUnauthorized {
		t.Fatalf("forged credential: %d", got.Code)
	}
	signIn := send(http.MethodPost, "/api/customer/google", `{"credential":"good"}`, "", true)
	cookies := signIn.Result().Cookies()
	if signIn.Code != http.StatusOK || len(cookies) != 1 {
		t.Fatalf("sign-in: %d %v", signIn.Code, cookies)
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != int(customers.SessionTTL.Seconds()) {
		t.Fatalf("cookie flags: %+v", cookie)
	}
	if strings.Contains(signIn.Body.String(), cookie.Value) {
		t.Fatal("session token leaked in the response body")
	}
	if got := send(http.MethodGet, "/api/customer/me", "", cookie.Value, false); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "rahim@example.test") {
		t.Fatalf("me: %d %s", got.Code, got.Body)
	}
	if got := send(http.MethodGet, "/api/customer/me", "", "", false); got.Code != http.StatusUnauthorized {
		t.Fatalf("me without a cookie: %d", got.Code)
	}
	logout := send(http.MethodPost, "/api/customer/logout", "", cookie.Value, true)
	if logout.Code != http.StatusNoContent || logout.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("logout: %d %v", logout.Code, logout.Result().Cookies())
	}
	if got := send(http.MethodGet, "/api/customer/me", "", cookie.Value, false); got.Code != http.StatusUnauthorized {
		t.Fatalf("old cookie after logout: %d", got.Code)
	}

	// Email + password: unknown account, expired proof, weak password, then sign-up and the profile.
	if got := send(http.MethodPost, "/api/customer/login", `{"email":"nobody@example.test","password":"long enough"}`, "", true); got.Code != http.StatusUnauthorized {
		t.Fatalf("unknown login: %d", got.Code)
	}
	if got := send(http.MethodPost, "/api/customer/password", `{"email_proof":"forged","password":"long enough"}`, "", true); got.Code != http.StatusBadRequest {
		t.Fatalf("forged proof: %d", got.Code)
	}
	proof, err := security.IssueEmailProof(secret, "new@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if got := send(http.MethodPost, "/api/customer/password", `{"email_proof":"`+proof+`","password":"short"}`, "", true); got.Code != http.StatusBadRequest {
		t.Fatalf("short password: %d", got.Code)
	}
	signUp := send(http.MethodPost, "/api/customer/password", `{"email_proof":"`+proof+`","password":"long enough"}`, "", true)
	if signUp.Code != http.StatusOK || len(signUp.Result().Cookies()) != 1 || !signUp.Result().Cookies()[0].HttpOnly {
		t.Fatalf("sign-up: %d %v", signUp.Code, signUp.Result().Cookies())
	}
	if got := send(http.MethodPut, "/api/customer/profile", `{"name":"Rahim","phone":"01712345678"}`, "", true); got.Code != http.StatusUnauthorized {
		t.Fatalf("profile without a session: %d", got.Code)
	}
	session := signUp.Result().Cookies()[0].Value
	if got := send(http.MethodPut, "/api/customer/profile", `{"name":"Rahim","phone":"123"}`, session, true); got.Code != http.StatusBadRequest {
		t.Fatalf("bad phone: %d", got.Code)
	}
	if got := send(http.MethodPut, "/api/customer/profile", `{"name":"Rahim Uddin","phone":"+8801712345678"}`, session, true); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"phone":"01712345678"`) {
		t.Fatalf("profile: %d %s", got.Code, got.Body)
	}
}

// countAll is a limiter that never reaches its limit in these tests.
type countAll struct{}

func (*countAll) Count(context.Context, string) (int, error) { return 1, nil }
