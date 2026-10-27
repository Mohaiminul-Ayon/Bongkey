package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bongkey/backend/internal/delivery"
	"github.com/gin-gonic/gin"
)

type deliverySettingsStoreStub struct{ fee, outsideFee int }

// Get returns the stubbed fee to exercise the HTTP boundary.
func (s *deliverySettingsStoreStub) Get(context.Context) (delivery.Settings, error) {
	return delivery.Settings{DeliveryFee: s.fee, OutsideDhakaFee: s.outsideFee}, nil
}

// Update records only values accepted by the delivery service.
func (s *deliverySettingsStoreStub) Update(_ context.Context, fee, outsideFee int) (delivery.Settings, error) {
	s.fee = fee
	s.outsideFee = outsideFee
	return delivery.Settings{DeliveryFee: fee, OutsideDhakaFee: outsideFee}, nil
}

// TestDeliverySettingsHTTP rejects missing and invalid input without changing the stored fee.
func TestDeliverySettingsHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &deliverySettingsStoreStub{fee: 80, outsideFee: 130}
	handler := DeliverySettingsHandler{Service: delivery.Service{Store: store}}
	router := gin.New()
	router.GET("/api/delivery-settings", handler.Get)
	router.PUT("/api/admin/delivery-settings", handler.Update)
	for _, tc := range []struct {
		body string
		want int
		fee  int
	}{
		{`{}`, http.StatusBadRequest, 80},
		{`{"delivery_fee":80}`, http.StatusBadRequest, 80},
		{`{"delivery_fee":-1,"outside_dhaka_fee":130}`, http.StatusBadRequest, 80},
		{`{"delivery_fee":80,"outside_dhaka_fee":-1}`, http.StatusBadRequest, 80},
		{`{"delivery_fee":125,"outside_dhaka_fee":150}`, http.StatusOK, 125},
	} {
		request := httptest.NewRequest(http.MethodPut, "/api/admin/delivery-settings", strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != tc.want || store.fee != tc.fee || store.outsideFee != map[bool]int{true: 150, false: 130}[tc.fee == 125] {
			t.Fatalf("body=%s status=%d fees=%d/%d, want status=%d Dhaka=%d", tc.body, response.Code, store.fee, store.outsideFee, tc.want, tc.fee)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/delivery-settings", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"delivery_fee":125`) || !strings.Contains(response.Body.String(), `"outside_dhaka_fee":150`) || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("public delivery settings status=%d body=%s", response.Code, response.Body.String())
	}
}
