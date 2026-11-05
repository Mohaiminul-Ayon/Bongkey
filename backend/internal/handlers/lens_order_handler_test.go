package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bongkey/backend/internal/database"
	"bongkey/backend/internal/orders"
	"github.com/gin-gonic/gin"
)

// multipartOrder builds a checkout body with the order JSON and the given upload fields.
func multipartOrder(t *testing.T, files map[string][]byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	order := `{"customer_name":"QA","customer_phone":"000","shipping_address":"Local","city":"Dhaka","expected_delivery_fee":80,
		"items":[{"product_id":1,"quantity":1,"lens_kind":"package","lens_package_id":7,"lens_power":"with_power"}]}`
	if err := writer.WriteField("order", order); err != nil {
		t.Fatal(err)
	}
	for field, data := range files {
		part, err := writer.CreateFormFile(field, "prescription.pdf")
		if err != nil {
			t.Fatal(err)
		}
		part.Write(data)
	}
	writer.Close()
	return &body, writer.FormDataContentType()
}

// TestCheckoutRejectsBadPrescriptionUploads refuses unsafe or misplaced files before the checkout service runs.
func TestCheckoutRejectsBadPrescriptionUploads(t *testing.T) {
	previous := database.DB
	database.DB = &sql.DB{}
	t.Cleanup(func() { database.DB = previous })
	gin.SetMode(gin.TestMode)
	store := &countingCheckoutStore{}
	router := gin.New()
	router.POST("/orders", CheckoutHandler{Service: orders.CheckoutService{Store: store}}.CreateOrder)
	for name, files := range map[string]map[string][]byte{
		"html pretending to be a pdf": {"prescription_0": []byte("<html><script>alert(1)</script>")},
		"file for a missing item":     {"prescription_3": []byte("%PDF-1.7")},
		"unexpected field name":       {"avatar": []byte("%PDF-1.7")},
		"too large":                   {"prescription_0": append([]byte("%PDF-"), make([]byte, 5<<20)...)},
	} {
		body, contentType := multipartOrder(t, files)
		request := httptest.NewRequest(http.MethodPost, "/orders", body)
		request.Header.Set("Content-Type", contentType)
		request.Header.Set("Idempotency-Key", "checkout-upload-test-0001")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || store.calls != 0 {
			t.Fatalf("%s: status=%d calls=%d body=%s", name, response.Code, store.calls, response.Body.String())
		}
	}
}

type countingCheckoutStore struct{ calls int }

// InTransaction counts calls; these tests expect the handler to stop before reaching storage.
func (s *countingCheckoutStore) InTransaction(_ context.Context, _ func(orders.CheckoutTx) error) error {
	s.calls++
	return orders.ErrUnavailable
}

// TestLensReviewRoutesRejectBadInput stops invalid IDs and values before the review store.
func TestLensReviewRoutesRejectBadInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := LensReviewHandler{}
	router.PATCH("/orders/:id/items/:itemId/prescription-status", handler.SetPrescriptionStatus)
	router.PATCH("/orders/:id/items/:itemId/lens-price", handler.SetLensPrice)
	router.POST("/orders/:id/advance-received", handler.MarkAdvanceReceived)
	router.POST("/orders/:id/items/:itemId/prescription", handler.AttachPrescription)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPatch, "/orders/1/items/1/prescription-status", `{"status":"accepted","note":"` + strings.Repeat("x", 501) + `"}`},
		{http.MethodPost, "/orders/1/items/1/prescription", ``},
		{http.MethodPatch, "/orders/x/items/1/prescription-status", `{"status":"accepted"}`},
		{http.MethodPatch, "/orders/1/items/0/prescription-status", `{"status":"accepted"}`},
		{http.MethodPatch, "/orders/1/items/1/prescription-status", `{"status":"maybe"}`},
		{http.MethodPatch, "/orders/1/items/1/lens-price", `{}`},
		{http.MethodPatch, "/orders/1/items/1/lens-price", `{"price":-5}`},
		{http.MethodPatch, "/orders/1/items/1/lens-price", `{"price":200001}`},
		{http.MethodPost, "/orders/-1/advance-received", ``},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s %.40s: status=%d", tc.method, tc.path, tc.body, response.Code)
		}
	}
	// An attached file is checked by content like a checkout upload.
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", "rx.pdf")
	part.Write([]byte("<html><script>alert(1)</script>"))
	form.Close()
	request := httptest.NewRequest(http.MethodPost, "/orders/1/items/1/prescription", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("html attached as a prescription: status=%d", response.Code)
	}
}
