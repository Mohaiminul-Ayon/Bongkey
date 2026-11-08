package middleware

import (
	"bongkey/backend/internal/security"
	"github.com/gin-gonic/gin"
)

// RequestIDKey is the gin context key holding this request's ID.
const RequestIDKey = "request_id"

// RequestID gives every request a fresh random ID, returns it in the X-Request-ID header and stores it
// under RequestIDKey for the access log and Recovery. It never rejects a request.
// It must run first so the logger and Recovery, which run after it, can read the ID.
// An incoming X-Request-ID is ignored: a client-chosen value could forge or garble log lines.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := security.RandomHex(8)
		if err != nil {
			id = "unavailable"
		}
		c.Set(RequestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}
