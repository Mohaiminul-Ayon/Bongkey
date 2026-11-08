package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"bongkey/backend/internal/orders"
	"github.com/gin-gonic/gin"
)

// OrderFulfillmentHandler maps owner status commands to the fulfillment use case.
type OrderFulfillmentHandler struct {
	Service  orders.FulfillmentService
	Tracking orders.TrackingService
	Emails   orders.EmailConfirmationService
}

// SetEmailConfirmed saves the staff tick that the customer's email was read back on the confirmation call.
func (h OrderFulfillmentHandler) SetEmailConfirmed(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<10)
	var req struct {
		Confirmed *bool `json:"confirmed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Confirmed == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmed must be true or false"})
		return
	}
	err = h.Emails.SetEmailConfirmed(c.Request.Context(), orderID, *req.Confirmed)
	c.Header("Cache-Control", "no-store")
	switch {
	case errors.Is(err, orders.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save the email check"})
	default:
		c.JSON(http.StatusOK, gin.H{"email_confirmed": *req.Confirmed})
	}
}

// SetTracking saves the courier name, tracking code and link the owner types for an order.
func (h OrderFulfillmentHandler) SetTracking(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<10)
	var req struct {
		Courier      string `json:"courier"`
		TrackingCode string `json:"tracking_code"`
		TrackingURL  string `json:"tracking_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tracking details"})
		return
	}
	err = h.Tracking.SetTracking(c.Request.Context(), orderID, req.Courier, req.TrackingCode, req.TrackingURL)
	c.Header("Cache-Control", "no-store")
	switch {
	case errors.Is(err, orders.ErrInvalidTracking):
		c.JSON(http.StatusBadRequest, gin.H{"error": "courier up to 60 characters, code up to 80, and the link must start with https://"})
	case errors.Is(err, orders.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
	case errors.Is(err, orders.ErrTrackingClosed):
		c.JSON(http.StatusConflict, gin.H{"error": "confirm the order first; cancelled orders have no tracking"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save tracking"})
	default:
		c.JSON(http.StatusOK, gin.H{"saved": true})
	}
}

// ChangeStatus validates the admin request and returns the committed order state.
func (h OrderFulfillmentHandler) ChangeStatus(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<10)
	var request struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}
	status, err := h.Service.ChangeStatus(c.Request.Context(), orderID, request.Status)
	switch {
	case errors.Is(err, orders.ErrInvalidOrderStatus):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order status"})
	case errors.Is(err, orders.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
	case errors.Is(err, orders.ErrLensPriceNotFinal):
		c.JSON(http.StatusConflict, gin.H{"error": "confirm the lens price before confirming the order"})
	case errors.Is(err, orders.ErrPrescriptionNotAccepted):
		c.JSON(http.StatusConflict, gin.H{"error": "accept the prescription before confirming the order"})
	case errors.Is(err, orders.ErrAdvanceNotReceived):
		c.JSON(http.StatusConflict, gin.H{"error": "record the advance payment before confirming the order"})
	case errors.Is(err, orders.ErrInvalidOrderTransition), errors.Is(err, orders.ErrFulfillmentConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "order status changed; reload and retry"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update order status"})
	default:
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"order_status": status})
	}
}
