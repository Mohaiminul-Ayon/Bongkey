package middleware

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"bongkey/backend/internal/auth"
	"bongkey/backend/internal/security"
	"github.com/gin-gonic/gin"
)

type adminReader struct {
	admin auth.Admin
	err   error
}

// ByEmail is unused by middleware and satisfies the admin repository boundary.
func (r adminReader) ByEmail(context.Context, string) (auth.Admin, error) {
	return auth.Admin{}, sql.ErrNoRows
}

// ByID supplies current account state for a protected request.
func (r adminReader) ByID(context.Context, int64) (auth.Admin, error) { return r.admin, r.err }

// TestAdminAuthAbortsInvalidRequests checks that protected handlers never run after rejection.
func TestAdminAuthAbortsInvalidRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "synthetic-test-secret-at-least-thirty-two-characters"
	token, err := security.IssueAdminToken(secret, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		header string
		reader adminReader
		status int
	}{
		{"missing", "", adminReader{}, http.StatusUnauthorized},
		{"malformed", "Bearer malformed", adminReader{}, http.StatusUnauthorized},
		{"deleted", "Bearer " + token, adminReader{err: sql.ErrNoRows}, http.StatusUnauthorized},
		{"wrong current role", "Bearer " + token, adminReader{admin: auth.Admin{ID: 4, Role: "customer"}}, http.StatusUnauthorized},
		{"database failure", "Bearer " + token, adminReader{err: errors.New("private database detail")}, http.StatusServiceUnavailable},
		{"owner", "Bearer " + token, adminReader{admin: auth.Admin{ID: 4, Role: "owner"}}, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := auth.Service{Repository: tc.reader, JWTSecret: secret}
			router := gin.New()
			called := false
			router.GET("/api/admin/me", AdminAuth(service), func(c *gin.Context) {
				called = true
				if c.GetInt64("admin_id") != 4 {
					t.Fatal("trusted admin identity missing")
				}
				c.Status(http.StatusOK)
			})
			request := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
			request.Header.Set("Authorization", tc.header)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status || called != (tc.status == http.StatusOK) {
				t.Fatalf("status=%d called=%v", response.Code, called)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("admin response can be cached")
			}
			if tc.name == "database failure" && response.Body.String() == "private database detail" {
				t.Fatal("database details exposed")
			}
		})
	}
}
