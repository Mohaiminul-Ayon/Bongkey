package handlers

import (
	"net/http"

	"bongkey/backend/internal/orders"
	"github.com/gin-gonic/gin"
)

// OrderListHandler returns owner order summaries after AdminAuth.
type OrderListHandler struct{ Reader orders.PostgresReader }

// ListOrders reads the order list without running SQL in the HTTP layer.
func (h OrderListHandler) ListOrders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	list, err := h.Reader.ListAdmin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load orders"})
		return
	}
	c.JSON(http.StatusOK, list)
}
