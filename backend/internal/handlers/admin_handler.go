package handlers

import (
	"errors"
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

// Login parses credentials and maps the admin use case result to HTTP.
func (h AdminHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	admin, token, err := h.Service.Login(c.Request.Context(), req.Email, req.Password)
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
