package handlers

import (
	"errors"
	"net/http"

	"bongkey/backend/internal/delivery"
	"github.com/gin-gonic/gin"
)

// DeliverySettingsHandler exposes public checkout settings and protected admin updates.
type DeliverySettingsHandler struct{ Service delivery.Service }

// Get returns the current delivery charge needed by cart totals.
func (h DeliverySettingsHandler) Get(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	settings, err := h.Service.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "delivery settings unavailable"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

type deliverySettingsRequest struct {
	DeliveryFee     *int `json:"delivery_fee"`
	OutsideDhakaFee *int `json:"outside_dhaka_fee"`
}

// Update validates and saves an admin-selected delivery charge.
func (h DeliverySettingsHandler) Update(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var req deliverySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.DeliveryFee == nil || req.OutsideDhakaFee == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "both delivery fees are required"})
		return
	}
	settings, err := h.Service.Update(c.Request.Context(), *req.DeliveryFee, *req.OutsideDhakaFee)
	if errors.Is(err, delivery.ErrInvalidFee) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "both fees must be between 0 and 1000000"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "delivery settings unavailable"})
		return
	}
	c.JSON(http.StatusOK, settings)
}
