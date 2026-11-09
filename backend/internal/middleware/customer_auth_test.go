package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bongkey/backend/internal/customers"
	"bongkey/backend/internal/security"

	"github.com/gin-gonic/gin"
)

// sessionStore knows one session token; only SessionCustomer is used by these tests.
type sessionStore struct {
	customers.Store
	token string
	err   error
}

func (s sessionStore) SessionCustomer(_ context.Context, tokenHash string) (customers.Customer, error) {
	if s.err != nil {
		return customers.Customer{}, s.err
	}
	if tokenHash != security.SessionTokenHash(s.token) {
		return customers.Customer{}, customers.ErrNoSession
	}
	return customers.Customer{ID: 7, Email: "a@example.test"}, nil
}

// TestOptionalCustomer checks checkout is never blocked and only a header-backed valid session is attached.
func TestOptionalCustomer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, tc := range map[string]struct {
		cookie, header string
		storeErr       error
		wantID         int64
	}{
		"signed in":         {cookie: "good", header: "bongkey", wantID: 7},
		"guest":             {},
		"no shop header":    {cookie: "good"},
		"unknown session":   {cookie: "bad", header: "bongkey"},
		"store unavailable": {cookie: "good", header: "bongkey", storeErr: errors.New("db down")},
	} {
		t.Run(name, func(t *testing.T) {
			router := gin.New()
			router.POST("/orders", OptionalCustomer(customers.Service{Store: sessionStore{token: "good", err: tc.storeErr}}), func(c *gin.Context) {
				var id int64
				if customer, ok := c.Get("customer"); ok {
					id = customer.(customers.Customer).ID
				}
				c.JSON(http.StatusOK, gin.H{"id": id})
			})
			req := httptest.NewRequest(http.MethodPost, "/orders", nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: CustomerCookie, Value: tc.cookie, Expires: time.Now().Add(time.Hour)})
			}
			if tc.header != "" {
				req.Header.Set("X-Requested-With", tc.header)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			want := `{"id":0}`
			if tc.wantID != 0 {
				want = `{"id":7}`
			}
			if rec.Code != http.StatusOK || rec.Body.String() != want {
				t.Fatalf("got %d %s, want 200 %s", rec.Code, rec.Body.String(), want)
			}
		})
	}
}
