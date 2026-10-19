package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"bongkey/backend/internal/catalog"
	"bongkey/backend/internal/config"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// TestAdminProductEditRejectsStaleStockPostgres protects a sale and accepts a refreshed edit.
func TestAdminProductEditRejectsStaleStockPostgres(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for local PostgreSQL stock QA")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("stock QA accepts only a local PostgreSQL URL")
	}
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL")
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error("could not close local PostgreSQL")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var categoryID, productID int
	if err := db.QueryRowContext(ctx, `SELECT id FROM categories ORDER BY id LIMIT 1`).Scan(&categoryID); err != nil {
		t.Fatal("local catalog has no category fixture")
	}
	slug := fmt.Sprintf("qa-admin-stock-%d-%d", os.Getpid(), time.Now().UnixNano())
	if err := db.QueryRowContext(ctx, `INSERT INTO products (category_id, name, slug, price, stock_quantity) VALUES ($1, 'QA admin stock', $2, 100, 2) RETURNING id`, categoryID, slug).Scan(&productID); err != nil {
		t.Fatal("could not create local product fixture")
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DELETE FROM products WHERE id = $1`, productID); err != nil {
			t.Error("could not remove local product fixture")
		}
	})
	if _, err := db.ExecContext(ctx, `UPDATE products SET stock_quantity = 1 WHERE id = $1`, productID); err != nil {
		t.Fatal("could not simulate a sale")
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	products := ProductHandler{Service: catalog.ProductService{Repository: catalog.PostgresProductRepository{DB: db}}}
	router.PUT("/api/admin/products/:id", products.UpdateProduct)
	body := fmt.Sprintf(`{"category_id":%d,"name":"QA admin stock","slug":"%s","price":100,"stock_quantity":5,"expected_stock_quantity":2}`, categoryID, slug)
	request := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/products/%d", productID), strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var remaining int
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, productID).Scan(&remaining); err != nil {
		t.Fatal("could not read local product stock")
	}
	if response.Code != http.StatusConflict || remaining != 1 {
		t.Fatalf("stale admin edit status=%d remaining=%d", response.Code, remaining)
	}
	freshBody := fmt.Sprintf(`{"category_id":%d,"name":"QA admin stock","slug":"%s","price":100,"stock_quantity":5,"expected_stock_quantity":1}`, categoryID, slug)
	freshRequest := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/products/%d", productID), strings.NewReader(freshBody))
	freshRequest.Header.Set("Content-Type", "application/json")
	freshResponse := httptest.NewRecorder()
	router.ServeHTTP(freshResponse, freshRequest)
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, productID).Scan(&remaining); err != nil {
		t.Fatal("could not read refreshed product stock")
	}
	if freshResponse.Code != http.StatusOK || remaining != 5 {
		t.Fatalf("refreshed edit status=%d remaining=%d", freshResponse.Code, remaining)
	}
}
