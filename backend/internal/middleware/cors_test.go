package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// preflight sends one browser preflight for checkout from the given origin.
func preflight(origin string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS([]string{"http://localhost:3000"}))
	router.POST("/api/orders", func(c *gin.Context) { c.Status(http.StatusOK) })
	request := httptest.NewRequest(http.MethodOptions, "/api/orders", nil)
	request.Header.Set("Origin", origin)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

// TestCORSAllowsCheckoutIdempotencyHeader keeps browser preflight aligned with checkout for the shop's frontend.
func TestCORSAllowsCheckoutIdempotencyHeader(t *testing.T) {
	response := preflight("http://localhost:3000")
	if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" ||
		!strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "Idempotency-Key") ||
		!strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "X-Requested-With") ||
		response.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("preflight status=%d headers=%v", response.Code, response.Header())
	}
}

// TestCORSIgnoresOtherOrigins gives other websites no permission to read API responses.
func TestCORSIgnoresOtherOrigins(t *testing.T) {
	response := preflight("https://evil.example")
	if got := response.Header().Get("Access-Control-Allow-Origin") + response.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("other origin allowed: %q", got)
	}
}
