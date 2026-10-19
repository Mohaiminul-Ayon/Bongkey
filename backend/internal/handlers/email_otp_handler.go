package handlers

import (
	"errors"
	"net"
	"net/http"

	"bongkey/backend/internal/emailotp"
	"github.com/gin-gonic/gin"
)

type emailOTPRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// EmailOTPHandler maps bounded OTP requests and verification to HTTP.
type EmailOTPHandler struct {
	Service emailotp.Service
}

// Request sends one code after persistent mailbox and direct-peer IP rate checks.
func (h EmailOTPHandler) Request(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
	var input emailOTPRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email is required"})
		return
	}
	// RemoteAddr ignores untrusted forwarded headers until the proxy boundary is configured.
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request source"})
		return
	}
	if err := h.Service.Request(c.Request.Context(), input.Email, ip); err != nil {
		switch {
		case errors.Is(err, emailotp.ErrInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "valid email is required"})
		case errors.Is(err, emailotp.ErrLimited):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "wait before requesting another code"})
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "could not send email code"})
		}
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "verification code sent"})
}

// Verify consumes one code and returns only a short-lived email proof.
func (h EmailOTPHandler) Verify(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
	var input emailOTPRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid verification code"})
		return
	}
	proof, err := h.Service.Verify(c.Request.Context(), input.Email, input.Code)
	if errors.Is(err, emailotp.ErrInvalid) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired verification code"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "email verification unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"email_proof": proof})
}
