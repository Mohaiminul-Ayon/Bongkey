package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"bongkey/backend/internal/callbacks"
	"github.com/gin-gonic/gin"
)

// CallbackHandler takes call-back requests from the lens panel and lists them for the owner.
type CallbackHandler struct {
	Service callbacks.Service
}

// Create saves a shopper's call-back request. Only name, phone, product_slug and message are read.
func (h CallbackHandler) Create(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req struct {
		Name        string `json:"name"`
		Phone       string `json:"phone"`
		ProductSlug string `json:"product_slug"`
		Message     string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enter your name, mobile number and lens needs"})
		return
	}
	saved, err := h.Service.Create(c.Request.Context(), callbacks.Request{Name: req.Name, Phone: req.Phone, ProductSlug: req.ProductSlug, Message: req.Message}, c.ClientIP())
	switch {
	case errors.Is(err, callbacks.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "enter your name, a Bangladesh mobile number (01XXXXXXXXX) and a few words (up to 500 characters) about your lens needs"})
	case errors.Is(err, callbacks.ErrLimited):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "we already have your request; please wait for our call or message us on WhatsApp"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not send your request"})
	default:
		// Only the ID goes back; the shopper already knows what they typed.
		c.JSON(http.StatusCreated, gin.H{"id": saved.ID})
	}
}

// AdminList returns call-back requests for the owner, optionally one status (?status=open).
func (h CallbackHandler) AdminList(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	list, err := h.Service.List(c.Request.Context(), c.Query("status"))
	switch {
	case errors.Is(err, callbacks.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be open or done"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load call-back requests"})
	default:
		c.JSON(http.StatusOK, list)
	}
}

// MarkDone records that staff called the shopper.
func (h CallbackHandler) MarkDone(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	err = h.Service.MarkDone(c.Request.Context(), id)
	switch {
	case errors.Is(err, callbacks.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update the request"})
	default:
		c.JSON(http.StatusOK, gin.H{"saved": true})
	}
}
