package handlers

import (
	"errors"
	"net/http"

	"bongkey/backend/internal/database"
	"bongkey/backend/internal/orders"

	"github.com/gin-gonic/gin"
)

type orderRequest struct {
	CustomerName        string             `json:"customer_name"`
	CustomerPhone       string             `json:"customer_phone"`
	CustomerEmail       string             `json:"customer_email"`
	EmailProof          string             `json:"email_proof"`
	ShippingAddress     string             `json:"shipping_address"`
	City                string             `json:"city"`
	DeliveryNote        string             `json:"delivery_note"`
	ExpectedDeliveryFee *int               `json:"expected_delivery_fee"`
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
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if !orders.ValidIdempotencyKey(idempotencyKey) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid idempotency key is required"})
		return
	}
	items := make([]orders.CheckoutItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = orders.CheckoutItem{ProductID: item.ProductID, Quantity: item.Quantity, LensOption: item.LensOption}
	}
	receipt, err := h.Service.Place(c.Request.Context(), orders.CheckoutRequest{
		IdempotencyKey: idempotencyKey,
		CustomerName:   req.CustomerName, CustomerPhone: req.CustomerPhone,
		CustomerEmail: req.CustomerEmail, EmailProof: req.EmailProof, ShippingAddress: req.ShippingAddress,
		City: req.City, DeliveryNote: req.DeliveryNote,
		ExpectedDeliveryFee: req.ExpectedDeliveryFee,
		PrescriptionFileURL: req.PrescriptionFileURL, Items: items,
	})
	if err != nil {
		switch {
		case errors.Is(err, orders.ErrInvalidCheckout):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order data"})
		case errors.Is(err, orders.ErrUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "item unavailable or out of stock"})
		case errors.Is(err, orders.ErrIdempotencyConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "idempotency key was reused with different order data"})
		case errors.Is(err, orders.ErrDeliveryUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "delivery settings unavailable"})
		case errors.Is(err, orders.ErrDeliveryFeeChanged):
			c.JSON(http.StatusConflict, gin.H{"error": "delivery charge changed; review the updated total and submit again"})
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
