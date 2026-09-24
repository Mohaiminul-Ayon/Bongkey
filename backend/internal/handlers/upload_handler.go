package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxUploadSize = 5 << 20

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

var allowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

type UploadHandler struct {
	RootDir string
}

func (h UploadHandler) ProductImage(c *gin.Context) {
	h.uploadImage(c, "products")
}

func (h UploadHandler) TryOnMockup(c *gin.Context) {
	h.uploadImage(c, "try-on")
}

func (h UploadHandler) uploadImage(c *gin.Context, folder string) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}
	if file.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image must be 5MB or smaller"})
		return
	}
	originalExtension := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExtensions[originalExtension] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only jpg, jpeg, png, and webp images are allowed"})
		return
	}

	opened, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read image"})
		return
	}
	defer opened.Close()

	header := make([]byte, 512)
	n, _ := opened.Read(header)
	contentType := http.DetectContentType(header[:n])
	extension, ok := allowedImageTypes[contentType]
	if !ok && isWebP(header[:n]) {
		extension = ".webp"
		ok = true
	}
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only jpg, jpeg, png, and webp images are allowed"})
		return
	}
	if _, err := opened.Seek(0, 0); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read image"})
		return
	}

	filename := uniqueFilename(file.Filename, extension)
	targetDir := filepath.Join(h.RootDir, "backend", "uploads", folder)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not prepare upload folder"})
		return
	}

	targetPath := filepath.Join(targetDir, filename)
	out, err := os.Create(targetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save image"})
		return
	}
	defer out.Close()

	if _, err := out.ReadFrom(opened); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save image"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": absoluteUploadURL(c, folder, filename)})
}

func uniqueFilename(originalName string, extension string) string {
	base := strings.TrimSuffix(filepath.Base(originalName), filepath.Ext(originalName))
	base = safeFilename(base)
	if base == "" {
		base = "image"
	}

	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("%s-%d%s", base, time.Now().UnixNano(), extension)
	}

	return fmt.Sprintf("%s-%s%s", base, hex.EncodeToString(randomBytes), extension)
}

func safeFilename(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	previousDash := false
	for _, char := range value {
		isSafe := char >= 'a' && char <= 'z' || char >= '0' && char <= '9'
		if isSafe {
			builder.WriteRune(char)
			previousDash = false
			continue
		}
		if !previousDash {
			builder.WriteRune('-')
			previousDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func isWebP(header []byte) bool {
	return len(header) >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WEBP"
}

func absoluteUploadURL(c *gin.Context, folder string, filename string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := c.GetHeader("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = forwardedProto
	}
	return fmt.Sprintf("%s://%s/uploads/%s/%s", scheme, c.Request.Host, folder, filename)
}
