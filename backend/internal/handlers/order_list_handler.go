package handlers

import (
	"net/http"
	"strconv"

	"bongkey/backend/internal/orders"
	"github.com/gin-gonic/gin"
)

// OrderListHandler returns owner order summaries after AdminAuth.
type OrderListHandler struct{ Reader orders.PostgresReader }

// ListOrders reads one page of the order list; ?before=<id> continues after the last order shown.
// Optional search, status, from and to narrow the list and must be resent with before.
func (h OrderListHandler) ListOrders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	beforeID := 0
	if raw := c.Query("before"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid before cursor"})
			return
		}
		beforeID = parsed
	}
	filter, err := orders.NewAdminOrderFilter(c.Query("search"), c.Query("status"), c.Query("from"), c.Query("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order filter"})
		return
	}
	list, err := h.Reader.ListAdmin(c.Request.Context(), beforeID, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load orders"})
		return
	}
	c.JSON(http.StatusOK, list)
}
