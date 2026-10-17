package middleware

import (
	"errors"
	"net/http"
	"strings"

	"bongkey/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

// AdminAuth requires a current owner session after CORS and supplies admin_id to protected handlers.
// It aborts invalid requests before the handler; database failures return a generic server error.
func AdminAuth(service auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin login required"})
			return
		}
		admin, err := service.Authenticate(c.Request.Context(), token)
		if errors.Is(err, auth.ErrInvalidSession) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid admin session"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "admin session unavailable"})
			return
		}
		c.Set("admin_id", admin.ID)
		c.Next()
	}
}
