package handlers

import (
	"errors"
	"net/http"
	"strings"

	"bongkey/backend/internal/database"
	"bongkey/backend/internal/orders"

	"github.com/gin-gonic/gin"
)

type orderRequest struct {
	CustomerName        string             `json:"customer_name"`
	CustomerPhone       string             `json:"customer_phone"`
	CustomerEmail       string             `json:"customer_email"`
	ShippingAddress     string             `json:"shipping_address"`
	City                string             `json:"city"`
	DeliveryNote        string             `json:"delivery_note"`
	PrescriptionFileURL string             `json:"prescription_file_url"`
	Items               []orderItemRequest `json:"items"`
}

type orderItemRequest struct {
	ProductID  int    `json:"product_id"`
	Quantity   int    `json:"quantity"`
	LensOption string `json:"lens_option"`
}

// CheckoutHandler maps order input and checkout decisions to HTTP.
type CheckoutHandler struct {
	Service orders.CheckoutService
}

// CreateOrder places a COD order without exposing persistence details to HTTP.
func (h CheckoutHandler) CreateOrder(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if !database.Ready() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not connected"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var req orderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order data"})
		return
	}
	if req.CustomerName == "" || req.CustomerPhone == "" || req.ShippingAddress == "" || req.City == "" || len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "customer details and cart items are required"})
		return
	}
	items := make([]orders.CheckoutItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = orders.CheckoutItem{ProductID: item.ProductID, Quantity: item.Quantity, LensOption: item.LensOption}
	}
	receipt, err := h.Service.Place(c.Request.Context(), orders.CheckoutRequest{
		CustomerName: req.CustomerName, CustomerPhone: req.CustomerPhone,
		CustomerEmail: req.CustomerEmail, ShippingAddress: req.ShippingAddress,
		City: req.City, DeliveryNote: req.DeliveryNote,
		PrescriptionFileURL: req.PrescriptionFileURL, Items: items,
	})
	if err != nil {
		switch {
		case errors.Is(err, orders.ErrInvalidCheckout):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order data"})
		case errors.Is(err, orders.ErrUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "item unavailable or out of stock"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create order"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": receipt.ID, "order_number": receipt.OrderNumber,
		"invoice_number": receipt.InvoiceNumber,
		"customer_name":  req.CustomerName, "customer_phone": req.CustomerPhone,
		"shipping_address": req.ShippingAddress, "city": req.City,
		"subtotal": receipt.Subtotal, "delivery_fee": receipt.DeliveryFee,
		"total": receipt.Total, "payment_method": "cash_on_delivery",
		"order_status": "pending", "guest_access_token": receipt.GuestAccessToken,
	})
}
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

// OrderAccessHandler maps private order access decisions to HTTP responses.
type OrderAccessHandler struct {
	Service orders.AccessService
}

// GetOrder requires the secret issued when this guest order was created.
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
	c.JSON(http.StatusOK, gin.H{
		"id": order.ID, "order_number": order.OrderNumber, "invoice_number": order.InvoiceNumber,
		"customer_name": order.CustomerName, "customer_phone": order.CustomerPhone,
		"shipping_address": order.ShippingAddress, "city": order.City,
		"subtotal": order.Subtotal, "delivery_fee": order.DeliveryFee,
		"total": order.Total, "order_status": order.Status,
	})
}
