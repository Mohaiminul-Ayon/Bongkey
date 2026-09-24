package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bongkey/backend/internal/orders"
	"github.com/gin-gonic/gin"
)

type stubOrderService struct{ err error }

// Create returns controlled outcomes to test HTTP error handling.
func (s stubOrderService) Create(context.Context, orders.Input) (orders.Created, error) {
	return orders.Created{}, s.err
}

// GetGuest returns controlled outcomes without querying customer data.
func (s stubOrderService) GetGuest(context.Context, string, string) (orders.Order, error) {
	return orders.Order{ID: 1}, s.err
}

// TestOrderErrorResponses verifies status mapping and prevents database details leaking to clients.
func TestOrderErrorResponses(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{nil, 200}, {orders.ErrCredentials, 401}, {orders.ErrNotFound, 404},
		{orders.ErrInvalid, 400}, {orders.ErrUnavailable, 400}, {errors.New("synthetic-private-db-diagnostic"), 500},
	} {
		router := gin.New()
		router.GET("/orders/:id", OrderHandler{Service: stubOrderService{err: tc.err}}.Get)
		request := httptest.NewRequest(http.MethodGet, "/orders/1", nil)
		request.Header.Set("Authorization", "Bearer test")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != tc.status || recorder.Header().Get("Cache-Control") != "no-store" || strings.Contains(recorder.Body.String(), "synthetic-private") {
			t.Fatalf("unsafe response: %d %s", recorder.Code, recorder.Body.String())
		}
	}
}

// TestCheckoutBodyLimit rejects oversized and malformed payloads before calling business logic.
func TestCheckoutBodyLimit(t *testing.T) {
	router := gin.New()
	router.POST("/orders", OrderHandler{}.Create)
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"customer_name":"` + strings.Repeat("x", 70*1024) + `"}`, 413}, {`{broken`, 400},
	} {
		request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != tc.status {
			t.Fatalf("got %d, want %d", recorder.Code, tc.status)
		}
	}
}
