package middleware

import (
	"errors"
	"net/http"

	"bongkey/backend/internal/customers"
	"github.com/gin-gonic/gin"
)

// CustomerCookie holds the customer session token. It is HttpOnly, so page scripts never see it.
const CustomerCookie = "bongkey_customer"

// CustomerCSRF rejects state-changing customer requests that lack the shop's custom header.
// A cross-site form cannot set a custom header, and a cross-site script needs a CORS preflight that
// only the shop's own origins pass, so the session cookie cannot be used by another site.
// It runs before CustomerAuth and before the sign-in handler (which would otherwise allow login CSRF).
func CustomerCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if c.Request.Method != http.MethodGet && c.GetHeader("X-Requested-With") != "bongkey" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "request blocked"})
			return
		}
		c.Next()
	}
}

// OptionalCustomer supplies "customer" (customers.Customer) when the request carries a current session cookie
// and the shop's X-Requested-With header; otherwise the request continues as a guest. It never rejects:
// checkout must work signed out. The header check keeps a cross-site form from filing an order under
// someone else's account, and a session-store failure falls back to a guest order rather than losing the sale.
func OptionalCustomer(service customers.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.Cookie(CustomerCookie)
		if token != "" && c.GetHeader("X-Requested-With") == "bongkey" {
			if customer, err := service.Authenticate(c.Request.Context(), token); err == nil {
				c.Set("customer", customer)
			}
		}
		c.Next()
	}
}

// CustomerAuth requires a current customer session cookie and supplies "customer" (customers.Customer) to handlers.
// It aborts with 401 when the cookie is missing, unknown or expired, and 503 when the session store fails.
// A customer session never satisfies AdminAuth: the two use different credentials and tables.
func CustomerAuth(service customers.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.Cookie(CustomerCookie)
		customer, err := service.Authenticate(c.Request.Context(), token)
		if errors.Is(err, customers.ErrNoSession) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "please sign in"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "sign-in unavailable"})
			return
		}
		c.Set("customer", customer)
		c.Next()
	}
}
