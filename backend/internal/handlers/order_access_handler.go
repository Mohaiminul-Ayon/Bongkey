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
	// SiteURL is printed on the order slip so the customer knows where to track the order.
	SiteURL string
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
	if errors.Is(err, orders.ErrInvoiceNotReady) {
		c.JSON(http.StatusConflict, gin.H{"error": "your invoice is ready after we confirm your order by phone; download the order slip for now"})
		return
	}
	respondWithInvoice(c, order, err)
}

// GetSlip downloads the order slip (order number, products, total, how to track) after order-scoped guest
// authorization. It is not an invoice: before confirmation the price or the order can still change.
func (h OrderAccessHandler) GetSlip(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || token == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	order, err := h.Service.GuestDetail(c.Request.Context(), c.Param("id"), token)
	if errors.Is(err, orders.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load the order"})
		return
	}
	data, err := invoice.RenderSlip(order, h.SiteURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create the order slip"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="order-%s.pdf"`, order.OrderNumber))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/pdf", data)
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

// TrackOrder is the public tracking lookup. The phone travels in the POST body, never in the URL.
// A wrong number or phone and a missing order get the same 404 so the page cannot be used to probe orders.
func (h OrderAccessHandler) TrackOrder(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<10)
	var req struct {
		OrderNumber string `json:"order_number"`
		Phone       string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enter your order number and phone"})
		return
	}
	view, err := h.Service.Track(c.Request.Context(), req.OrderNumber, req.Phone, c.ClientIP())
	switch {
	case errors.Is(err, orders.ErrTrackLimited):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many tries; please wait 15 minutes"})
	case errors.Is(err, orders.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no order matches this order number and phone"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load the order"})
	default:
		c.JSON(http.StatusOK, view)
	}
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
		"advance_mode": order.AdvanceMode, "advance_required": order.AdvanceRequired, "amount_paid": order.AmountPaid,
		"tracking": order.Tracking, "email_confirmed": order.EmailConfirmed, "risk_flags": order.RiskFlags,
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
		"lens_order": order.LensOrder, "pay_in_advance": order.PayInAdvance, "lens_price_pending": order.LensPricePending,
		"advance_required": order.AdvanceRequired, "amount_paid": order.AmountPaid,
		"tracking": order.Tracking,
	})
}
