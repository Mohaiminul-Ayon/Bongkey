package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bongkey/backend/internal/emailotp"
	"github.com/gin-gonic/gin"
)

type httpCodeStore struct {
	hash       string
	used       bool
	reserveErr error
}

// Reserve captures the challenge hash for HTTP response checks.
func (s *httpCodeStore) Reserve(_ context.Context, _, _, hash string) error {
	s.hash = hash
	return s.reserveErr
}

// Release marks a test code unusable after failed delivery.
func (s *httpCodeStore) Release(_ context.Context, _, _ string) error { s.used = true; return nil }

// Consume accepts the captured hash once.
func (s *httpCodeStore) Consume(_ context.Context, _, hash string) (bool, error) {
	if s.used || hash != s.hash {
		return false, nil
	}
	s.used = true
	return true, nil
}

type httpCodeSender struct {
	code string
	err  error
}

// Send captures a synthetic code without contacting Gmail.
func (s *httpCodeSender) Send(_ context.Context, _, code string) error { s.code = code; return s.err }

// TestEmailOTPHTTP keeps the code out of responses and rejects reuse.
func TestEmailOTPHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, sender := &httpCodeStore{}, &httpCodeSender{}
	handler := EmailOTPHandler{Service: emailotp.Service{Store: store, Sender: sender, Secret: "synthetic-email-proof-secret-at-least-32-characters"}}
	router := gin.New()
	router.POST("/request", handler.Request)
	router.POST("/verify", handler.Verify)
	call := func(path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	if response := call("/request", `{"email":"buyer@example.com"}`); response.Code != http.StatusAccepted || response.Header().Get("Cache-Control") != "no-store" || sender.code == "" || strings.Contains(response.Body.String(), sender.code) {
		t.Fatal("request failed or exposed the code")
	}
	body, err := json.Marshal(map[string]string{"email": "buyer@example.com", "code": sender.code})
	if err != nil {
		t.Fatal(err)
	}
	if response := call("/verify", string(body)); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "email_proof") {
		t.Fatal("verified code did not return a proof")
	}
	if response := call("/verify", string(body)); response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "email_proof") {
		t.Fatal("reused code returned private proof")
	}
}

// TestEmailOTPRequestFailures rejects abuse before sending and hides provider failure details.
func TestEmailOTPRequestFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name                string
		reserveErr, sendErr error
		status              int
	}{
		{"limited", emailotp.ErrLimited, nil, http.StatusTooManyRequests},
		{"provider failure", nil, errors.New("synthetic private SMTP detail"), http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &httpCodeStore{reserveErr: tc.reserveErr}
			sender := &httpCodeSender{err: tc.sendErr}
			handler := EmailOTPHandler{Service: emailotp.Service{Store: store, Sender: sender, Secret: "synthetic-email-proof-secret-at-least-32-characters"}}
			router := gin.New()
			router.POST("/request", handler.Request)
			request := httptest.NewRequest(http.MethodPost, "/request", strings.NewReader(`{"email":"buyer@example.com"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status || response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), "synthetic private") {
				t.Fatal("failure status, cache policy, or provider detail redaction is incorrect")
			}
			if tc.reserveErr != nil && sender.code != "" {
				t.Fatal("rate-limited request reached the email provider")
			}
			if tc.sendErr != nil && !store.used {
				t.Fatal("failed delivery left its challenge usable")
			}
		})
	}
}
