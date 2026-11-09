package handlers

import (
	"errors"
	"net/http"

	"bongkey/backend/internal/customers"
	"bongkey/backend/internal/middleware"
	"bongkey/backend/internal/security"
	"github.com/gin-gonic/gin"
)

// CustomerHandler maps customer sign-in, session and order-link requests to the customers service.
type CustomerHandler struct {
	Service customers.Service
	// SecureCookie is false only in local development, where the site runs on plain http.
	SecureCookie bool
}

// setSession writes (or, with maxAge -1, clears) the HttpOnly, SameSite=Lax session cookie.
func (h CustomerHandler) setSession(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.CustomerCookie, token, maxAge, "/", "", h.SecureCookie, true)
}

// SignIn exchanges a Google ID token for a session cookie.
func (h CustomerHandler) SignIn(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req struct {
		Credential string `json:"credential"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Credential == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google sign-in data is missing"})
		return
	}
	customer, token, err := h.Service.SignIn(c.Request.Context(), req.Credential)
	switch {
	case errors.Is(err, customers.ErrNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google sign-in is not available yet"})
	case errors.Is(err, customers.ErrSignInFailed):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Google sign-in failed; please try again"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not sign in"})
	default:
		h.setSession(c, token, int(customers.SessionTTL.Seconds()))
		c.JSON(http.StatusOK, customer)
	}
}

// Login exchanges an email and password for a session cookie.
func (h CustomerHandler) Login(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enter your email and password"})
		return
	}
	customer, token, err := h.Service.Login(c.Request.Context(), req.Email, req.Password, c.ClientIP())
	switch {
	case errors.Is(err, customers.ErrLoginFailed):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wrong email or password"})
	case errors.Is(err, customers.ErrLoginLimited):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many tries; please wait 15 minutes or reset your password"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not sign in"})
	default:
		h.setSession(c, token, int(customers.SessionTTL.Seconds()))
		c.JSON(http.StatusOK, customer)
	}
}

// SetPassword creates an account or resets its password using the proof from the email-code step, then signs in.
func (h CustomerHandler) SetPassword(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10)
	var req struct {
		EmailProof string `json:"email_proof"`
		Password   string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	customer, token, err := h.Service.SetPassword(c.Request.Context(), req.EmailProof, req.Password)
	switch {
	case errors.Is(err, customers.ErrInvalidProof):
		c.JSON(http.StatusBadRequest, gin.H{"error": "your email code has expired; please ask for a new one"})
	case errors.Is(err, security.ErrWeakPassword):
		c.JSON(http.StatusBadRequest, gin.H{"error": "use a password of 8 to 72 characters"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save your password"})
	default:
		h.setSession(c, token, int(customers.SessionTTL.Seconds()))
		c.JSON(http.StatusOK, customer)
	}
}

// UpdateProfile saves the signed-in customer's name and mobile number.
func (h CustomerHandler) UpdateProfile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	customer, err := h.Service.UpdateProfile(c.Request.Context(), c.MustGet("customer").(customers.Customer), req.Name, req.Phone)
	switch {
	case errors.Is(err, customers.ErrInvalidName):
		c.JSON(http.StatusBadRequest, gin.H{"error": "enter your name (2 to 80 characters)"})
	case errors.Is(err, customers.ErrInvalidPhone):
		c.JSON(http.StatusBadRequest, gin.H{"error": "enter a valid Bangladeshi mobile number"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save your profile"})
	default:
		c.JSON(http.StatusOK, customer)
	}
}

// Logout ends the session server-side and clears the cookie, even if the session had already expired.
func (h CustomerHandler) Logout(c *gin.Context) {
	token, _ := c.Cookie(middleware.CustomerCookie)
	if err := h.Service.Logout(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not sign out"})
		return
	}
	h.setSession(c, "", -1)
	c.Status(http.StatusNoContent)
}

// Me returns the signed-in customer that CustomerAuth attached.
func (h CustomerHandler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, c.MustGet("customer").(customers.Customer))
}

// Orders lists the signed-in customer's linked orders.
func (h CustomerHandler) Orders(c *gin.Context) {
	list, err := h.Service.Orders(c.Request.Context(), c.MustGet("customer").(customers.Customer))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load your orders"})
		return
	}
	c.JSON(http.StatusOK, list)
}

// LinkCandidates says how many earlier guest orders carry the customer's email.
func (h CustomerHandler) LinkCandidates(c *gin.Context) {
	count, err := h.Service.LinkCandidates(c.Request.Context(), c.MustGet("customer").(customers.Customer))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check earlier orders"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

// Link attaches earlier guest orders whose email and phone both match.
func (h CustomerHandler) Link(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<10)
	var req struct {
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enter the phone number you used"})
		return
	}
	linked, err := h.Service.Link(c.Request.Context(), c.MustGet("customer").(customers.Customer), req.Phone)
	switch {
	case errors.Is(err, customers.ErrInvalidPhone):
		c.JSON(http.StatusBadRequest, gin.H{"error": "enter a valid Bangladeshi mobile number"})
	case errors.Is(err, customers.ErrLinkLimited):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many tries; please wait 15 minutes"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not link orders"})
	default:
		c.JSON(http.StatusOK, gin.H{"linked": linked})
	}
}
