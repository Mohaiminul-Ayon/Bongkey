package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bongkey/backend/internal/config"
	"github.com/gin-gonic/gin"
)

// TestSetupRouterIgnoresForwardedClientIP verifies that untrusted headers cannot spoof the client address.
func TestSetupRouterIgnoresForwardedClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := SetupRouter(config.Config{})
	router.GET("/test-client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	request := httptest.NewRequest(http.MethodGet, "/test-client-ip", nil)
	request.RemoteAddr = "192.0.2.10:4567"
	request.Header.Set("X-Forwarded-For", "203.0.113.20")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "192.0.2.10" {
		t.Fatalf("client IP = %q, status = %d; want direct peer address", response.Body.String(), response.Code)
	}
}

// TestDeliverySettingsUpdateRequiresAdmin proves unauthenticated writes stop in middleware.
func TestDeliverySettingsUpdateRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := SetupRouter(config.Config{})
	request := httptest.NewRequest(http.MethodPut, "/api/admin/delivery-settings", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated delivery update status=%d", response.Code)
	}
}

// TestAdminOrderDetailsRequiresAdmin proves the detail route rejects requests before its reader runs.
func TestAdminOrderDetailsRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := SetupRouter(config.Config{})
	request := httptest.NewRequest(http.MethodGet, "/api/admin/orders/1", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated order detail status=%d", response.Code)
	}
}

// TestAdminInvoiceRequiresAdmin checks the PDF route's middleware boundary.
func TestAdminInvoiceRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	SetupRouter(config.Config{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/admin/orders/1/invoice.pdf", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated invoice status=%d", response.Code)
	}
}

// TestGuestInvoiceRouteRequiresProof checks that the registered route denies anonymous reads.
func TestGuestInvoiceRouteRequiresProof(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	SetupRouter(config.Config{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/orders/1/invoice.pdf", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("anonymous invoice status=%d", response.Code)
	}
}

// TestCreateVariantRequiresAdmin ensures the protected route stops before storage access.
func TestCreateVariantRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := SetupRouter(config.Config{})
	request := httptest.NewRequest(http.MethodPost, "/api/admin/products/1/variants", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated variant create status=%d", response.Code)
	}
}
