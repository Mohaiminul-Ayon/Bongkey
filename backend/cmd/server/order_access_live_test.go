package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"bongkey/backend/internal/config"
	"bongkey/backend/internal/security"
)

// TestLiveOrderAccessQA verifies required email, retry safety, and private order access through the local server.
func TestLiveOrderAccessQA(t *testing.T) {
	if os.Getenv("RUN_LOCAL_ORDER_HTTP_QA") != "1" {
		t.Skip("set RUN_LOCAL_ORDER_HTTP_QA=1 with the local backend running")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("local app configuration is not ready")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || cfg.AppEnv != "development" || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		t.Fatal("order access QA requires a development database on localhost")
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal("local PostgreSQL is not reachable with app credentials")
	}

	marker := fmt.Sprintf("QA-FIX01-%d-%d", os.Getpid(), time.Now().UnixNano())
	slug := strings.ToLower(marker)
	mailbox := slug + "@example.invalid"
	proof, err := security.IssueEmailProof(cfg.JWTSecret, mailbox)
	if err != nil {
		t.Fatal("could not issue synthetic email proof")
	}
	var productID int
	if err := db.QueryRowContext(ctx, `
		INSERT INTO products (name, slug, price, stock_quantity, is_active)
		VALUES ($1, $2, 100, 2, true) RETURNING id
	`, marker, slug).Scan(&productID); err != nil {
		t.Fatal("could not create local QA product")
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		tx, err := db.BeginTx(cleanupCtx, nil)
		if err != nil {
			t.Error("could not begin QA cleanup")
			return
		}
		defer tx.Rollback()
		// Delete only orders placed for this unique fixture; dependent items and invoices cascade.
		if _, err := tx.ExecContext(cleanupCtx, `
			DELETE FROM orders WHERE customer_name = $1 AND EXISTS (
				SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND product_id = $2
			)
		`, marker, productID); err != nil {
			t.Error("could not remove local QA orders")
			return
		}
		result, err := tx.ExecContext(cleanupCtx, `DELETE FROM products WHERE id = $1 AND slug = $2`, productID, slug)
		if err != nil {
			t.Error("could not remove local QA product")
			return
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			t.Error("local QA product cleanup was incomplete")
			return
		}
		if err := tx.Commit(); err != nil {
			t.Error("could not commit local QA cleanup")
		}
	})

	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := "http://127.0.0.1:" + cfg.ServerPort
	check := func(path, authorization string, want int, private bool) {
		t.Helper()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
		if err != nil {
			t.Fatal("could not prepare local request")
		}
		if authorization != "" {
			request.Header.Set("Authorization", "Bearer "+authorization)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal("local backend is not reachable")
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal("could not read local response")
		}
		if response.StatusCode != want || response.Header.Get("Cache-Control") != "no-store" || bytes.Contains(body, []byte(marker)) != private {
			t.Fatalf("path=%s status=%d, want=%d; private data visible=%v", path, response.StatusCode, want, bytes.Contains(body, []byte(marker)))
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/products/"+slug, nil)
	if err != nil {
		t.Fatal("could not prepare catalog check")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal("local backend is not reachable")
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal("running backend does not see the local QA product")
	}
	missingEmailPayload := []byte(fmt.Sprintf(`{"customer_name":%q,"customer_phone":"0000000000","shipping_address":"QA only","city":"Dhaka","items":[{"product_id":%d,"quantity":1}]}`, marker, productID))
	missingEmailRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/orders", bytes.NewReader(missingEmailPayload))
	if err != nil {
		t.Fatal("could not prepare unverified checkout")
	}
	missingEmailRequest.Header.Set("Content-Type", "application/json")
	missingEmailRequest.Header.Set("Idempotency-Key", "qa-email-missing-0001")
	missingEmailResponse, err := client.Do(missingEmailRequest)
	if err != nil {
		t.Fatal("local unverified checkout request failed")
	}
	missingEmailBody, readErr := io.ReadAll(io.LimitReader(missingEmailResponse.Body, 64<<10))
	closeErr := missingEmailResponse.Body.Close()
	if readErr != nil || closeErr != nil || missingEmailResponse.StatusCode != http.StatusBadRequest || bytes.Contains(missingEmailBody, []byte("guest_access_token")) {
		t.Fatal("checkout without verified email was not rejected safely")
	}

	type receipt struct {
		ID               int    `json:"id"`
		OrderNumber      string `json:"order_number"`
		GuestAccessToken string `json:"guest_access_token"`
	}
	var orders [2]receipt
	var firstPayload []byte
	var firstKey string
	for i := range orders {
		payload, err := json.Marshal(map[string]any{
			"customer_name": marker, "customer_phone": "0000000000",
			"customer_email": mailbox, "email_proof": proof,
			"shipping_address": "QA only", "city": "Dhaka",
			"items": []map[string]int{{"product_id": productID, "quantity": 1}},
		})
		if err != nil {
			t.Fatal("could not prepare QA order")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/orders", bytes.NewReader(payload))
		if err != nil {
			t.Fatal("could not prepare checkout request")
		}
		request.Header.Set("Content-Type", "application/json")
		key := fmt.Sprintf("live-order-access-%d-%d", time.Now().UnixNano(), i)
		request.Header.Set("Idempotency-Key", key)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal("local checkout request failed")
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal("could not read checkout response")
		}
		if response.StatusCode != http.StatusOK || json.Unmarshal(body, &orders[i]) != nil || orders[i].ID <= 0 || orders[i].GuestAccessToken == "" {
			t.Fatalf("local checkout %d did not return a valid receipt (status=%d)", i+1, response.StatusCode)
		}
		if i == 0 {
			firstPayload, firstKey = payload, key
		}
	}
	for _, tc := range []struct {
		name    string
		payload []byte
		want    int
	}{
		{"same checkout retry", firstPayload, http.StatusOK},
		{"changed checkout retry", bytes.Replace(firstPayload, []byte(`"Dhaka"`), []byte(`"Chattogram"`), 1), http.StatusConflict},
	} {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/orders", bytes.NewReader(tc.payload))
		if err != nil {
			t.Fatal("could not prepare checkout retry")
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", firstKey)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal("local checkout retry failed")
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != tc.want {
			t.Fatalf("%s status=%d, want=%d (read=%v close=%v)", tc.name, response.StatusCode, tc.want, readErr, closeErr)
		}
		if tc.want == http.StatusOK {
			var retry receipt
			if json.Unmarshal(body, &retry) != nil || retry != orders[0] {
				t.Fatal("identical checkout retry returned a different receipt")
			}
		} else if !bytes.Contains(body, []byte("idempotency key was reused")) || bytes.Contains(body, []byte("guest_access_token")) {
			t.Fatal("conflicting checkout retry was not rejected safely")
		}
	}
	var remaining, orderCount, invoiceCount int
	if err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM products WHERE id = $1`, productID).Scan(&remaining); err != nil {
		t.Fatal("could not read checkout QA stock")
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(i.id) FROM orders o LEFT JOIN invoices i ON i.order_id = o.id WHERE o.customer_name = $1`, marker).Scan(&orderCount, &invoiceCount); err != nil {
		t.Fatal("could not count checkout QA orders and invoices")
	}
	if remaining != 0 || orderCount != 2 || invoiceCount != 2 {
		t.Fatalf("checkout retries changed records: stock=%d orders=%d invoices=%d", remaining, orderCount, invoiceCount)
	}

	path := fmt.Sprintf("/api/orders/%d", orders[0].ID)
	check(path, "", http.StatusNotFound, false)
	check(path, "invalid", http.StatusNotFound, false)
	check(path, orders[1].GuestAccessToken, http.StatusNotFound, false)
	check(path, orders[0].GuestAccessToken, http.StatusOK, true)
	check(path, proof, http.StatusOK, true)
	check("/api/orders/"+orders[0].OrderNumber, orders[0].GuestAccessToken, http.StatusOK, true)

	var ownerID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM admins WHERE role = 'owner' ORDER BY id LIMIT 1`).Scan(&ownerID); err != nil {
		t.Fatal("local owner account is not available")
	}
	ownerToken, err := security.IssueAdminToken(cfg.JWTSecret, ownerID)
	if err != nil {
		t.Fatal("could not issue local owner QA token")
	}
	adminPath := fmt.Sprintf("/api/admin/orders/%d", orders[0].ID)
	check(adminPath, "", http.StatusUnauthorized, false)
	check(adminPath, "invalid", http.StatusUnauthorized, false)
	check(adminPath, ownerToken, http.StatusOK, true)
	t.Log("local required-email checkout, retry, conflict, and private order access passed; QA fixtures are removed by cleanup")
}
