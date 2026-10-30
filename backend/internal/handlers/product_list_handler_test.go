package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bongkey/backend/internal/catalog"
	"github.com/gin-gonic/gin"
)

// TestListProductsRejectsBadPriceQueries checks that malformed or invalid filters get a 400 before any database read.
func TestListProductsRejectsBadPriceQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// A repository without a database would answer 503, so a 400 proves the request stopped before storage.
	router := gin.New()
	router.GET("/api/products", ProductHandler{Service: catalog.ProductService{Repository: catalog.PostgresProductRepository{}}}.ListProducts)
	for _, test := range []struct {
		name, query string
		want        int
	}{
		{"text min price", "?min_price=abc", http.StatusBadRequest},
		{"decimal max price", "?max_price=12.5", http.StatusBadRequest},
		{"min above max", "?min_price=3000&max_price=1000", http.StatusBadRequest},
		{"unknown sort", "?sort=cheapest", http.StatusBadRequest},
		{"valid filter reaches storage", "?min_price=1000&max_price=2000&sort=price_asc", http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/products"+test.query, nil))
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body %s", response.Code, test.want, response.Body.String())
			}
		})
	}
}
