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

type unavailableCheckoutStore struct{}

// InTransaction simulates an atomic stock rejection without touching a database.
func (unavailableCheckoutStore) InTransaction(context.Context, func(orders.CheckoutTx) error) error {
	return orders.ErrUnavailable
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
	router := gin.New()
	router.POST("/api/orders", (CheckoutHandler{Service: orders.CheckoutService{Store: unavailableCheckoutStore{}, OrderTokenSecret: "synthetic-order-token-secret-at-least-32-characters"}}).CreateOrder)
	for _, tc := range []struct {
		name, quantity, key string
		want                int
	}{
		{"missing idempotency key", "1", "", http.StatusBadRequest},
		{"zero quantity", "0", "checkout-handler-zero-0001", http.StatusBadRequest},
		{"out of stock", "1", "checkout-handler-stock-0001", http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"customer_name":"QA","customer_phone":"000","customer_email":"buyer@example.com","email_proof":"` + proof + `","shipping_address":"Local","city":"Dhaka","expected_delivery_fee":80,"items":[{"product_id":1,"quantity":` + tc.quantity + `}]}`
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
