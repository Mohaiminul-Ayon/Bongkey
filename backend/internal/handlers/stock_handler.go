package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"bongkey/backend/internal/catalog"
	"github.com/gin-gonic/gin"
)

// StockHandler serves the owner's low-stock list and stock history; routes put it behind AdminAuth.
type StockHandler struct{ Service catalog.StockService }

// LowStock lists items at or below their alert level.
func (h StockHandler) LowStock(c *gin.Context) {
	items, err := h.Service.LowStock(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load low stock"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// History lists one item's stock changes.
func (h StockHandler) History(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	changes, err := h.Service.History(c.Request.Context(), id)
	if errors.Is(err, catalog.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load stock history"})
		return
	}
	c.JSON(http.StatusOK, changes)
}
