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
	Service orders.FulfillmentService
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
	case errors.Is(err, orders.ErrInvalidOrderTransition), errors.Is(err, orders.ErrFulfillmentConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "order status changed; reload and retry"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update order status"})
	default:
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"order_status": status})
	}
}
