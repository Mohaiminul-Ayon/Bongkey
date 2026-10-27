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

// Items supplies a synthetic saved line for the admin-only response.
func (r orderReader) Items(context.Context, int) ([]orders.Item, error) {
	return []orders.Item{{ProductName: "Private frame", Quantity: 1, UnitPrice: 100, TotalPrice: 100}}, nil
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
			CustomerEmail:  "private@example.com",
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
			if strings.Contains(response.Body.String(), "private@example.com") || strings.Contains(response.Body.String(), "Private frame") {
				t.Fatal("admin-only details leaked through guest endpoint")
			}
		})
	}
}

// TestAdminOrderResponse maps saved lines and missing orders at the HTTP boundary.
func TestAdminOrderResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := OrderAccessHandler{Service: orders.AccessService{Reader: orderReader{record: orders.Record{
		Summary:       orders.Summary{ID: 1, OrderNumber: "ORD-000001", CustomerPhone: "private-phone"},
		CustomerEmail: "private@example.com", DeliveryNote: "Call first", PaymentMethod: "cash_on_delivery", PaymentStatus: "unpaid",
	}}}}
	router := gin.New()
	router.GET("/api/admin/orders/:id", handler.GetAdminOrder)
	for _, tc := range []struct {
		id   string
		want int
	}{{"1", http.StatusOK}, {"2", http.StatusNotFound}} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/admin/orders/"+tc.id, nil))
		if response.Code != tc.want || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("id=%s status=%d", tc.id, response.Code)
		}
		if tc.want == http.StatusOK && (!strings.Contains(response.Body.String(), "Private frame") || !strings.Contains(response.Body.String(), "private@example.com")) {
			t.Fatalf("admin detail omitted saved item/contact: %s", response.Body.String())
		}
		if tc.want == http.StatusNotFound && strings.Contains(response.Body.String(), "private-phone") {
			t.Fatal("missing order response disclosed private data")
		}
	}
}

// TestGuestInvoiceResponse checks PDF delivery and denial without leaking private details.
func TestGuestInvoiceResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, hash, err := security.NewOrderToken()
	if err != nil {
		t.Fatal(err)
	}
	handler := OrderAccessHandler{Service: orders.AccessService{Reader: orderReader{record: orders.Record{
		Summary:        orders.Summary{ID: 1, OrderNumber: "ORD-1", InvoiceNumber: "INV-1", CustomerName: "Private buyer", Total: 180},
		GuestTokenHash: hash, PaymentMethod: "cash_on_delivery", PaymentStatus: "unpaid",
	}}}}
	router := gin.New()
	router.GET("/api/orders/:id/invoice.pdf", handler.GetInvoice)
	for _, tc := range []struct {
		name, id, auth string
		status         int
	}{
		{"missing token", "1", "", http.StatusNotFound},
		{"wrong token", "1", "Bearer wrong", http.StatusNotFound},
		{"other order", "2", "Bearer " + token, http.StatusNotFound},
		{"own invoice", "1", "Bearer " + token, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/orders/"+tc.id+"/invoice.pdf", nil)
			request.Header.Set("Authorization", tc.auth)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d cache=%q", response.Code, response.Header().Get("Cache-Control"))
			}
			if tc.status == http.StatusOK {
				if !strings.HasPrefix(response.Body.String(), "%PDF-") || response.Header().Get("Content-Type") != "application/pdf" {
					t.Fatal("authorized invoice is not a PDF")
				}
			} else if strings.Contains(response.Body.String(), "Private buyer") {
				t.Fatal("denied PDF disclosed customer data")
			}
		})
	}
}
