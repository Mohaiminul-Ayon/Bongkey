package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestListOrdersRejectsInvalidCursor stops malformed paging input before any database work.
func TestListOrdersRejectsInvalidCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/orders", OrderListHandler{}.ListOrders)
	for _, cursor := range []string{"abc", "0", "-5"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/orders?before="+cursor, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("cursor %q: status = %d, want 400", cursor, response.Code)
		}
	}
}
