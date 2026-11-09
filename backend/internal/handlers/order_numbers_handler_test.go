package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bongkey/backend/internal/orders"
	"github.com/gin-gonic/gin"
)

// oneEmailOrders has orders for exactly one email.
type oneEmailOrders struct{}

// OrdersByEmail returns one order for rahim@example.com and none otherwise.
func (oneEmailOrders) OrdersByEmail(_ context.Context, email string, _ int) ([]orders.EmailedOrder, error) {
	if email == "rahim@example.com" {
		return []orders.EmailedOrder{{OrderNumber: "ORD-000123"}}, nil
	}
	return nil, nil
}

// countingSender counts queued reminders.
type countingSender struct{ sent int }

// OrderNumbers counts one reminder.
func (s *countingSender) OrderNumbers(context.Context, string, string, []orders.EmailedOrder) {
	s.sent++
}

// limitAfter counts requests; the test raises seen to go over the limit.
type limitAfter struct{ seen int }

// Count adds one request.
func (l *limitAfter) Count(context.Context, string) (int, error) { l.seen++; return l.seen, nil }

// TestSendOrderNumbersHTTP returns the same 202 for known and unknown emails, 400 for a bad one, and 429 when limited.
func TestSendOrderNumbersHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sender := &countingSender{}
	limiter := &limitAfter{}
	router := gin.New()
	router.POST("/api/track/order-numbers", OrderAccessHandler{Numbers: orders.OrderNumberService{
		Orders: oneEmailOrders{}, Sender: sender, Limiter: limiter, Secret: strings.Repeat("s", 32),
	}}.SendOrderNumbers)
	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/track/order-numbers", strings.NewReader(body)))
		return w
	}

	known, unknown := post(`{"email":"rahim@example.com"}`), post(`{"email":"nobody@example.com"}`)
	if known.Code != http.StatusAccepted || unknown.Code != http.StatusAccepted || known.Body.String() != unknown.Body.String() {
		t.Fatalf("known=%d %s unknown=%d %s", known.Code, known.Body, unknown.Code, unknown.Body)
	}
	if sender.sent != 1 {
		t.Fatalf("expected one reminder, got %d", sender.sent)
	}
	if w := post(`{"email":"abc"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad email: %d", w.Code)
	}
	limiter.seen = 100
	if w := post(`{"email":"rahim@example.com"}`); w.Code != http.StatusTooManyRequests {
		t.Fatalf("limited: %d", w.Code)
	}
}
