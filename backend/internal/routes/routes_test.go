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
