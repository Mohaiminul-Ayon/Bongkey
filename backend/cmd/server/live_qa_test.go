package main

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/database"
	"bongkey/backend/internal/routes"
	"bongkey/backend/internal/security"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// TestLocalDatabaseAccessQA checks the real local schema and protected routes without changing records.
func TestLocalDatabaseAccessQA(t *testing.T) {
	if os.Getenv("RUN_LOCAL_DB_QA") != "1" {
		t.Skip("set RUN_LOCAL_DB_QA=1 for read-only local PostgreSQL QA")
	}
	cfg, err := config.Load()
	if err != nil {
		if strings.Contains(err.Error(), "JWT_SECRET") {
			t.Fatal("local JWT_SECRET is missing, shorter than 32 characters, or uses a rejected template value")
		}
		t.Fatal("local app configuration is not ready; check .env format privately")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("live QA accepts only a local PostgreSQL URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		t.Fatal("could not open local PostgreSQL connection")
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) {
			switch string(pgErr.Code) {
			case "28P01":
				t.Fatal("PostgreSQL rejected the configured password")
			case "3D000":
				t.Fatal("the configured PostgreSQL database does not exist")
			case "28000":
				t.Fatal("PostgreSQL rejected the configured user or host access")
			}
		}
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			port := parsed.Port()
			if port == "" {
				port = "5432"
			}
			t.Fatalf("the configured local PostgreSQL port %s is not reachable", port)
		}
		t.Fatal("local PostgreSQL connection failed; check DATABASE_URL privately")
	}

	var adminID int64
	var passwordHash string
	if err := db.QueryRowContext(ctx, `SELECT id, password_hash FROM admins WHERE role = 'owner' ORDER BY id LIMIT 1`).Scan(&adminID, &passwordHash); err != nil {
		t.Fatal("local database has no readable owner account")
	}
	if _, err := bcrypt.Cost([]byte(passwordHash)); err != nil {
		t.Error("existing owner password hash is invalid")
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte("admin12345")) == nil {
		t.Error("existing owner still uses the old demo password")
	}
	token, err := security.IssueAdminToken(cfg.JWTSecret, adminID)
	if err != nil {
		t.Fatal("could not issue a local QA admin token")
	}
	previous := database.DB
	database.DB = db
	defer func() { database.DB = previous }()
	gin.SetMode(gin.TestMode)
	router := routes.SetupRouter(cfg)
	for _, tc := range []struct {
		name, path, authorization string
		status                    int
	}{
		{"missing admin token", "/api/admin/me", "", http.StatusUnauthorized},
		{"bad admin token", "/api/admin/me", "Bearer malformed", http.StatusUnauthorized},
		{"current owner", "/api/admin/me", "Bearer " + token, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.authorization != "" {
				request.Header.Set("Authorization", tc.authorization)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status=%d, want=%d", response.Code, tc.status)
			}
		})
	}
	rows, err := db.QueryContext(ctx, `SELECT guest_token_hash FROM orders LIMIT 0`)
	if err != nil {
		t.Fatal("guest order access migration is not applied")
	}
	if err := rows.Close(); err != nil {
		t.Fatal("could not close schema check")
	}
	var orderID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM orders ORDER BY id DESC LIMIT 1`).Scan(&orderID); err == sql.ErrNoRows {
		t.Log("no existing order; live guest/admin order access remains unverified")
		return
	} else if err != nil {
		t.Fatal("could not query local orders")
	}
	path := "/api/orders/" + strconv.FormatInt(orderID, 10)
	for _, authorization := range []string{"", "Bearer invalid"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private order was not hidden from a guest without its token")
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/admin/orders/"+strconv.FormatInt(orderID, 10), nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatal("current owner could not read an existing order")
	}
}
