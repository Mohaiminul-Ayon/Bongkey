package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestProductImageUploadReturnsHostFreePath keeps stored image URLs valid when the API host changes.
func TestProductImageUploadReturnsHostFreePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	router := gin.New()
	router.POST("/upload", UploadHandler{RootDir: root}.ProductImage)

	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89")
	for _, tc := range []struct {
		name, filename string
		content        []byte
		status         int
	}{
		{"png", "Frame Photo.png", png, http.StatusOK},
		{"text renamed as png", "fake.png", []byte("not an image at all"), http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			part, err := form.CreateFormFile("image", tc.filename)
			if err != nil {
				t.Fatal(err)
			}
			part.Write(tc.content)
			form.Close()
			request := httptest.NewRequest(http.MethodPost, "/upload", &body)
			request.Host = "api.example.test"
			request.Header.Set("Content-Type", form.FormDataContentType())
			request.Header.Set("X-Forwarded-Proto", "javascript")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.status, response.Body.String())
			}
			if tc.status != http.StatusOK {
				return
			}
			var result struct{ URL string }
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(result.URL, "/uploads/products/frame-photo-") || strings.Contains(result.URL, "example.test") {
				t.Fatalf("upload URL must be a host-free path, got %q", result.URL)
			}
			if _, err := os.Stat(filepath.Join(root, "backend", filepath.FromSlash(result.URL))); err != nil {
				t.Fatalf("uploaded file missing: %v", err)
			}
		})
	}
}
