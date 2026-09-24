package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bongkey/backend/internal/config"
)

// TestOrderRouteRejectsMissingCredentials checks the real route chain without a database.
func TestOrderRouteRejectsMissingCredentials(t *testing.T) {
	router := SetupRouter(config.Config{ProjectRoot: t.TempDir()}, nil)
	for _, header := range []string{"", "Basic test", "Bearer", "Bearer malformed", "Bearer a b"} {
		request := httptest.NewRequest(http.MethodGet, "/api/orders/ORD-000001", nil)
		request.Header.Set("Authorization", header)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized || recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("unsafe response: %d %s", recorder.Code, recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), "customer") {
			t.Fatal("private information exposed")
		}
	}
}
