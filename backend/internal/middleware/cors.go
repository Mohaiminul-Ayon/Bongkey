package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

// CORS answers preflight requests and lets only the shop's own frontend origins read API responses.
// It is not authentication: other origins get no Access-Control-Allow-Origin, so browsers block
// them from reading responses, while non-browser clients are still stopped by AdminAuth and order tokens.
// It runs before auth so preflight requests never reach protected handlers.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		if origin := c.GetHeader("Origin"); origin != "" && slices.Contains(allowedOrigins, origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, Idempotency-Key")
			// The shop's pages may read the request ID to show it next to an error.
			c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
