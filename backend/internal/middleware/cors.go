package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

// CORS answers preflight requests and lets only the shop's own frontend origins read API responses.
// It is not authentication: other origins get no Access-Control-Allow-Origin, so browsers block
// them from reading responses or sending the customer cookie, while non-browser clients are still stopped by
// AdminAuth, CustomerAuth and order tokens.
// It runs before auth so preflight requests never reach protected handlers.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		if origin := c.GetHeader("Origin"); origin != "" && slices.Contains(allowedOrigins, origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, Idempotency-Key, X-Requested-With")
			// Credentials let the shop's own pages send the HttpOnly customer cookie; other origins still get nothing.
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
