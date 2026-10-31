package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestListOrdersRejectsInvalidFilters stops unusable filter values before any database work.
func TestListOrdersRejectsInvalidFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/orders", OrderListHandler{}.ListOrders)
	for _, query := range []url.Values{
		{"status": {"lost"}},
		{"from": {"31-10-2026"}},
		{"to": {"2026-02-30"}},
		{"from": {"2026-10-31"}, "to": {"2026-10-01"}},
		{"search": {strings.Repeat("1", 51)}},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/orders?"+query.Encode(), nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("query %q: status = %d, want 400", query.Encode(), response.Code)
		}
	}
}

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
