package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"bongkey/backend/internal/invoice"
	"bongkey/backend/internal/orders"
	"github.com/gin-gonic/gin"
)

// OrderAccessHandler maps private order access decisions to HTTP responses.
type OrderAccessHandler struct {
	Service orders.AccessService
}

// GetInvoice downloads a snapshot PDF after order-scoped guest authorization.
func (h OrderAccessHandler) GetInvoice(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || token == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	order, err := h.Service.GuestInvoice(c.Request.Context(), c.Param("id"), token)
	respondWithInvoice(c, order, err)
}

// GetAdminInvoice downloads the same snapshot after AdminAuth verifies the owner.
func (h OrderAccessHandler) GetAdminInvoice(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	order, err := h.Service.Admin(c.Request.Context(), c.Param("id"))
	respondWithInvoice(c, order, err)
}

// respondWithInvoice maps denied or failed reads to safe errors before PDF output starts.
func respondWithInvoice(c *gin.Context, order orders.AdminDetail, err error) {
	if errors.Is(err, orders.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load invoice"})
		return
	}
	data, err := invoice.Render(order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create invoice"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="invoice-%d.pdf"`, order.ID))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/pdf", data)
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
	if errors.Is(err, orders.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load order"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": order.ID, "order_number": order.OrderNumber, "invoice_number": order.InvoiceNumber,
		"customer_name": order.CustomerName, "customer_phone": order.CustomerPhone, "customer_email": order.CustomerEmail,
		"shipping_address": order.ShippingAddress, "city": order.City, "delivery_note": order.DeliveryNote,
		"subtotal": order.Subtotal, "delivery_fee": order.DeliveryFee, "total": order.Total,
		"order_status": order.Status, "payment_method": order.PaymentMethod, "payment_status": order.PaymentStatus,
		"created_at": order.CreatedAt, "items": order.Items,
	})
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
	c.JSON(http.StatusOK, gin.H{
		"id": order.ID, "order_number": order.OrderNumber, "invoice_number": order.InvoiceNumber,
		"customer_name": order.CustomerName, "customer_phone": order.CustomerPhone,
		"shipping_address": order.ShippingAddress, "city": order.City,
		"subtotal": order.Subtotal, "delivery_fee": order.DeliveryFee,
		"total": order.Total, "order_status": order.Status,
	})
}
