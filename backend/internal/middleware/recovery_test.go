package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRequestIDAndRecovery returns one ID in the header, the panic log and the 500 body, without request secrets.
func TestRequestIDAndRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(RequestID(), Recovery(&logs))
	router.GET("/ok", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/boom", func(*gin.Context) { panic("synthetic failure") })

	first, second := httptest.NewRecorder(), httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/ok", nil))
	router.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if id := first.Header().Get("X-Request-ID"); len(id) != 16 || id == second.Header().Get("X-Request-ID") {
		t.Fatalf("request IDs %q and %q", id, second.Header().Get("X-Request-ID"))
	}

	request := httptest.NewRequest(http.MethodGet, "/boom?phone=01712345678", nil)
	request.Header.Set("X-Request-ID", "client-chosen")
	request.Header.Set("Authorization", "Bearer secret-admin-token")
	request.AddCookie(&http.Cookie{Name: "bongkey_customer", Value: "secret-session"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	id := response.Header().Get("X-Request-ID")
	if response.Code != http.StatusInternalServerError || id == "client-chosen" || !strings.Contains(response.Body.String(), id) {
		t.Fatalf("panic response %d %q id=%q", response.Code, response.Body, id)
	}
	line := logs.String()
	if !strings.Contains(line, "request_id="+id) || !strings.Contains(line, "synthetic failure") {
		t.Fatalf("panic log missing the ID or cause: %q", line)
	}
	for _, secret := range []string{"secret-admin-token", "secret-session", "01712345678"} {
		if strings.Contains(line, secret) || strings.Contains(response.Body.String(), secret) {
			t.Fatalf("%q leaked", secret)
		}
	}
}
