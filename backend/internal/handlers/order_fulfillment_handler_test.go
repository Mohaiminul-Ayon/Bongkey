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

type handlerFulfillmentStore struct {
	status string
}

// InTransaction supplies one in-memory fulfillment transaction to the handler service.
func (store *handlerFulfillmentStore) InTransaction(_ context.Context, work func(orders.FulfillmentTx) error) error {
	return work(store)
}

// LockOrder returns the handler fixture's current state.
func (store *handlerFulfillmentStore) LockOrder(context.Context, int) (string, error) {
	return store.status, nil
}

// RestoreStock is a no-op because service tests cover restoration accounting.
func (*handlerFulfillmentStore) RestoreStock(context.Context, int) error { return nil }

// UpdateStatus applies the handler fixture transition.
func (store *handlerFulfillmentStore) UpdateStatus(_ context.Context, _ int, _, next string) error {
	store.status = next
	return nil
}

// TestOrderFulfillmentHandler maps accepted, invalid, and conflicting commands safely.
func TestOrderFulfillmentHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, current, body string
		wantStatus          int
	}{
		{"accepted", "pending", `{"status":"confirmed"}`, http.StatusOK},
		{"unknown status", "pending", `{"status":"refunded"}`, http.StatusBadRequest},
		{"invalid transition", "delivered", `{"status":"cancelled"}`, http.StatusConflict},
		{"invalid body", "pending", `{}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &handlerFulfillmentStore{status: tc.current}
			handler := OrderFulfillmentHandler{Service: orders.FulfillmentService{Store: store}}
			router := gin.New()
			router.PATCH("/orders/:id/status", handler.ChangeStatus)
			request := httptest.NewRequest(http.MethodPatch, "/orders/7/status", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
