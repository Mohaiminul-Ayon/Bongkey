package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bongkey/backend/internal/database"
	"bongkey/backend/internal/orders"
	"bongkey/backend/internal/security"
	"github.com/gin-gonic/gin"
)

type failingCheckoutStore struct{ err error }

// InTransaction simulates a checkout rejection without touching a database.
func (s failingCheckoutStore) InTransaction(context.Context, func(orders.CheckoutTx) error) error {
	return s.err
}

// TestCheckoutHTTPRejectsBadQuantityAndStock checks safe client responses.
func TestCheckoutHTTPRejectsBadQuantityAndStock(t *testing.T) {
	proof, err := security.IssueEmailProof("synthetic-email-proof-secret-at-least-32-characters", "buyer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = &sql.DB{}
	t.Cleanup(func() { database.DB = previous })
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, quantity, key string
		storeErr            error
		want                int
	}{
		{"missing idempotency key", "1", "", orders.ErrUnavailable, http.StatusBadRequest},
		{"zero quantity", "0", "checkout-handler-zero-0001", orders.ErrUnavailable, http.StatusBadRequest},
		{"out of stock", "1", "checkout-handler-stock-0001", orders.ErrUnavailable, http.StatusConflict},
		{"changed delivery fee", "1", "checkout-handler-fee-0001", orders.ErrDeliveryFeeChanged, http.StatusConflict},
		{"unknown district", "1", "checkout-handler-district-0001", orders.ErrInvalidCheckout, http.StatusBadRequest},
		{"delivery settings unreadable", "1", "checkout-handler-settings-0001", orders.ErrDeliveryUnavailable, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/orders", (CheckoutHandler{Service: orders.CheckoutService{Store: failingCheckoutStore{err: tc.storeErr}, OrderTokenSecret: "synthetic-order-token-secret-at-least-32-characters"}}).CreateOrder)
			body := `{"customer_name":"QA","customer_phone":"01712345678","customer_email":"buyer@example.com","email_proof":"` + proof + `","shipping_address":"Local","city":"Dhaka","expected_delivery_fee":80,"items":[{"product_id":1,"quantity":` + tc.quantity + `}]}`
			request := httptest.NewRequest(http.MethodPost, "/api/orders", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			if tc.key != "" {
				request.Header.Set("Idempotency-Key", tc.key)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.want || strings.Contains(response.Body.String(), "guest_access_token") {
				t.Fatalf("status=%d, want=%d; response disclosed token=%v", response.Code, tc.want, strings.Contains(response.Body.String(), "guest_access_token"))
			}
		})
	}
}

// TestCheckoutHTTPBotSigns drops a submission with both bot signs silently and asks a lone honeypot to try again.
func TestCheckoutHTTPBotSigns(t *testing.T) {
	previous := database.DB
	database.DB = &sql.DB{}
	t.Cleanup(func() { database.DB = previous })
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, extra string
		want        int
		received    bool
	}{
		{"honeypot and fast", `,"website":"http://spam.example","fill_ms":400`, http.StatusOK, true},
		{"honeypot only", `,"website":"autofilled","fill_ms":45000`, http.StatusBadRequest, false},
		// Speed alone reaches checkout (here the stub store reports no stock) and becomes a red flag, not a refusal.
		{"fast only", `,"fill_ms":400`, http.StatusConflict, false},
		{"normal", `,"fill_ms":45000`, http.StatusConflict, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/orders", (CheckoutHandler{Service: orders.CheckoutService{Store: failingCheckoutStore{err: orders.ErrUnavailable}, OrderTokenSecret: "synthetic-order-token-secret-at-least-32-characters"}}).CreateOrder)
			body := `{"customer_name":"QA","customer_phone":"01712345678","customer_email":"buyer@example.com","shipping_address":"Local","city":"Dhaka","expected_delivery_fee":80,"items":[{"product_id":1,"quantity":1}]` + tc.extra + `}`
			request := httptest.NewRequest(http.MethodPost, "/api/orders", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "checkout-handler-bot-0001")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.want || strings.Contains(response.Body.String(), `"received":true`) != tc.received {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
