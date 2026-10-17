package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bongkey/backend/internal/orders"
	"bongkey/backend/internal/security"

	"github.com/gin-gonic/gin"
)

type orderReader struct {
	record orders.Record
}

// Find supplies one private order only for its requested ID.
func (r orderReader) Find(_ context.Context, id string) (orders.Record, error) {
	if id != "1" {
		return orders.Record{}, orders.ErrNotFound
	}
	return r.record, nil
}

// TestGuestOrderResponse verifies that missing/wrong tokens never return personal data.
func TestGuestOrderResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, hash, err := security.NewOrderToken()
	if err != nil {
		t.Fatal(err)
	}
	handler := OrderAccessHandler{Service: orders.AccessService{Reader: orderReader{
		record: orders.Record{
			Summary:        orders.Summary{ID: 1, CustomerPhone: "private-phone", ShippingAddress: "private-address"},
			GuestTokenHash: hash,
		},
	}}}
	router := gin.New()
	router.GET("/api/orders/:id", handler.GetOrder)
	for _, tc := range []struct {
		name, id, auth string
		status         int
	}{
		{"missing", "1", "", http.StatusNotFound},
		{"wrong", "1", "Bearer invalid", http.StatusNotFound},
		{"other ID", "2", "Bearer " + token, http.StatusNotFound},
		{"own", "1", "Bearer " + token, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/orders/"+tc.id, nil)
			if tc.auth != "" {
				request.Header.Set("Authorization", tc.auth)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private responses must not be cached")
			}
			containsPrivateData := strings.Contains(response.Body.String(), "private-phone") || strings.Contains(response.Body.String(), "private-address")
			if containsPrivateData != (tc.status == http.StatusOK) {
				t.Fatalf("unexpected private data in response: %s", response.Body.String())
			}
		})
	}
}
