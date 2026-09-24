package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"bongkey/backend/internal/database"
	"bongkey/backend/internal/orders"
	"github.com/gin-gonic/gin"
)

type orderService interface {
	Create(context.Context, orders.Input) (orders.Created, error)
	GetGuest(context.Context, string, string) (orders.Order, error)
}

// OrderHandler adapts checkout and guest access to HTTP without owning persistence.
type OrderHandler struct{ Service orderService }

// Create parses bounded checkout input and returns a credential only after commit.
func (h OrderHandler) Create(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var input orders.Input
	if err := c.ShouldBindJSON(&input); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "order request too large"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order data"})
		}
		return
	}
	result, err := h.Service.Create(c.Request.Context(), input)
	if err != nil {
		writeOrderError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Get rejects absent credentials before the service can read private order fields.
func (h OrderHandler) Get(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		writeOrderError(c, orders.ErrCredentials)
		return
	}
	result, err := h.Service.GetGuest(c.Request.Context(), c.Param("id"), parts[1])
	if err != nil {
		writeOrderError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// writeOrderError maps expected failures and hides internal database diagnostics.
func writeOrderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, orders.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": orders.ErrInvalid.Error()})
	case errors.Is(err, orders.ErrUnavailable):
		c.JSON(http.StatusBadRequest, gin.H{"error": orders.ErrUnavailable.Error()})
	case errors.Is(err, orders.ErrCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": orders.ErrCredentials.Error()})
	case errors.Is(err, orders.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": orders.ErrNotFound.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not process order"})
	}
}

// ListOrders returns the inherited administrator order queue.
func ListOrders(c *gin.Context) {
	rows, err := database.DB.Query(`SELECT id, order_number, customer_name, customer_phone, city, total, order_status, created_at FROM orders ORDER BY id DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load orders"})
		return
	}
	defer rows.Close()
	var orders []gin.H
	for rows.Next() {
		var id, total int
		var orderNumber, name, phone, city, status, createdAt string
		if err := rows.Scan(&id, &orderNumber, &name, &phone, &city, &total, &status, &createdAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read orders"})
			return
		}
		orders = append(orders, gin.H{"id": id, "order_number": orderNumber, "customer_name": name, "customer_phone": phone, "city": city, "total": total, "order_status": status, "created_at": createdAt})
	}
	c.JSON(http.StatusOK, orders)
}
