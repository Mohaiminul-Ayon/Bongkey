package handlers

import (
	"errors"
	"net/http"
	"strings"

	"bongkey/backend/internal/orders"
	"github.com/gin-gonic/gin"
)

// OrderAccessHandler maps private order access decisions to HTTP responses.
type OrderAccessHandler struct {
	Service orders.AccessService
}

// GetOrder requires the order secret or a fresh proof of its verified email.
func (h OrderAccessHandler) GetOrder(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || token == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	order, err := h.Service.Guest(c.Request.Context(), c.Param("id"), token)
	respondWithOrder(c, order, err)
}

// GetAdminOrder returns private details after AdminAuth has accepted the request.
func (h OrderAccessHandler) GetAdminOrder(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	order, err := h.Service.Admin(c.Request.Context(), c.Param("id"))
	respondWithOrder(c, order, err)
}

// respondWithOrder keeps errors generic and excludes the stored access hash.
func respondWithOrder(c *gin.Context, order orders.Summary, err error) {
	if errors.Is(err, orders.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load order"})
		return
	}
	items := make([]gin.H, len(order.Items))
	for i, item := range order.Items {
		items[i] = gin.H{
			"product_name": item.ProductName, "lens_option": item.LensOption, "quantity": item.Quantity,
			"unit_price": item.UnitPrice, "total_price": item.TotalPrice,
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"id": order.ID, "order_number": order.OrderNumber, "invoice_number": order.InvoiceNumber,
		"customer_name": order.CustomerName, "customer_phone": order.CustomerPhone,
		"shipping_address": order.ShippingAddress, "city": order.City,
		"subtotal": order.Subtotal, "delivery_fee": order.DeliveryFee,
		"total": order.Total, "order_status": order.Status, "items": items,
	})
}
