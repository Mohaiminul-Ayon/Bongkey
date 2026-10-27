package handlers

import (
	"errors"
	"net"
	"net/http"

	"bongkey/backend/internal/auth"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	Service auth.Service
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login parses bounded credentials, passes the direct peer IP for rate limiting, and maps the result to HTTP.
func (h AdminHandler) Login(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	// RemoteAddr ignores untrusted forwarded headers until the proxy boundary is configured.
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request source"})
		return
	}
	admin, token, err := h.Service.Login(c.Request.Context(), req.Email, req.Password, ip)
	if errors.Is(err, auth.ErrLoginLimited) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts; wait 15 minutes"})
		return
	}
	if errors.Is(err, auth.ErrInvalidCredentials) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "login unavailable"})
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"admin": gin.H{"id": admin.ID, "name": admin.Name, "email": admin.Email},
	})
}

// Me confirms that the admin middleware accepted the current session.
func (h AdminHandler) Me(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"authenticated": true})
}
