package middleware

import (
	"fmt"
	"io"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery turns a handler panic into a generic 500 with the request ID, and logs the panic, the ID, the method,
// the path without its query, and the stack to out. Unlike gin.Recovery it never dumps request headers, so cookies,
// bearer tokens and other customer data stay out of logs. It needs RequestID before it and runs after the logger,
// so the 500 is still logged. Ordinary handler errors are not panics and pass through untouched.
// It only covers the request goroutine; a panic in another goroutine is not caught here.
func Recovery(out io.Writer) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			id := c.GetString(RequestIDKey)
			fmt.Fprintf(out, "[PANIC] request_id=%s %s %s: %v\n%s", id, c.Request.Method, c.Request.URL.Path, recovered, debug.Stack())
			if c.Writer.Written() {
				// Headers are already sent; the client gets a cut-off response, so only stop the chain.
				c.Abort()
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "something went wrong; please try again", "request_id": id})
		}()
		c.Next()
	}
}
