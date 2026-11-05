package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"bongkey/backend/internal/orders"
	"bongkey/backend/internal/payments"
	"github.com/gin-gonic/gin"
)

// PaymentHandler maps manual payment submissions and the owner's review to HTTP.
// Guest routes authorize with the same order secret as the order page; admin routes run after AdminAuth.
type PaymentHandler struct {
	Service payments.Service
	Orders  orders.AccessService
}

// guestOrderID checks the guest's order secret and returns the order's internal id; it answers 404 itself on failure.
func (h PaymentHandler) guestOrderID(c *gin.Context) (int, bool) {
	c.Header("Cache-Control", "no-store")
	token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || token == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return 0, false
	}
	order, err := h.Orders.Guest(c.Request.Context(), c.Param("id"), token)
	if errors.Is(err, orders.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return 0, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load order"})
		return 0, false
	}
	return order.ID, true
}

// GuestPayments shows the customer whether to pay now, the bKash number while payable, and what they sent.
func (h PaymentHandler) GuestPayments(c *gin.Context) {
	orderID, ok := h.guestOrderID(c)
	if !ok {
		return
	}
	view, err := h.Service.ForGuest(c.Request.Context(), orderID)
	if err != nil {
		respondPayment(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// SubmitPayment records the customer's bKash transaction as waiting for the owner's check.
func (h PaymentHandler) SubmitPayment(c *gin.Context) {
	orderID, ok := h.guestOrderID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<10)
	var req struct {
		Method       string `json:"method"`
		TrxID        string `json:"trx_id"`
		SenderNumber string `json:"sender_number"`
		Amount       int    `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "check the transaction ID, number and amount"})
		return
	}
	payment, err := h.Service.Submit(c.Request.Context(), orderID, payments.Submission{Method: req.Method, TrxID: req.TrxID, SenderNumber: req.SenderNumber, Amount: req.Amount})
	if err != nil {
		respondPayment(c, err)
		return
	}
	payment.UsedOnOrders = nil
	c.JSON(http.StatusCreated, payment)
}

// AdminPayments lists payments for the owner, optionally only one status (?status=pending).
func (h PaymentHandler) AdminPayments(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	list, err := h.Service.List(c.Request.Context(), c.Query("status"))
	if err != nil {
		respondPayment(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// AdminOrderPayments lists one order's payments with reuse warnings.
func (h PaymentHandler) AdminOrderPayments(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}
	list, err := h.Service.ForOrder(c.Request.Context(), orderID)
	if err != nil {
		respondPayment(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// ApprovePayment records the money as received after the owner found it in the bKash app.
func (h PaymentHandler) ApprovePayment(c *gin.Context) {
	paymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment id"})
		return
	}
	respondPaymentReview(c, h.Service.Approve(c.Request.Context(), paymentID, c.GetInt64("admin_id")))
}

// RejectPayment closes a payment the owner could not match, with a reason the customer sees.
func (h PaymentHandler) RejectPayment(c *gin.Context) {
	paymentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment id"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<10)
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a reason is required"})
		return
	}
	respondPaymentReview(c, h.Service.Reject(c.Request.Context(), paymentID, c.GetInt64("admin_id"), req.Reason))
}

// PaymentOptions tells the checkout page which online payments it may offer and the advance rules it uses
// to preview the amount due now; it never returns the receiving number. The server recomputes the amount.
func (h PaymentHandler) PaymentOptions(c *gin.Context) {
	enabled, err := h.Service.Enabled(c.Request.Context(), payments.MethodBkash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load payment options"})
		return
	}
	policies, err := h.Service.Policies(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load payment options"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bkash": enabled, "policies": policies})
}

// AdvancePolicies returns both advance rules for the owner's settings page.
func (h PaymentHandler) AdvancePolicies(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	policies, err := h.Service.Policies(c.Request.Context())
	if err != nil {
		respondPayment(c, err)
		return
	}
	c.JSON(http.StatusOK, policies)
}

// SaveAdvancePolicy changes the rule for future frame or lens orders.
func (h PaymentHandler) SaveAdvancePolicy(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<10)
	var req struct {
		Mode  string `json:"mode"`
		Value int    `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid advance rule"})
		return
	}
	err := h.Service.SavePolicy(c.Request.Context(), payments.Policy{Kind: c.Param("kind"), Mode: req.Mode, Value: req.Value})
	if errors.Is(err, payments.ErrInvalidPayment) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid advance rule: lens orders need full, a percent (1-100) or a fixed amount; frame orders need none, the delivery fee, a percent or a fixed amount"})
		return
	}
	respondPaymentReview(c, err)
}

// PaymentMethod returns one method's settings for the owner.
func (h PaymentHandler) PaymentMethod(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	method, err := h.Service.Method(c.Request.Context(), c.Param("code"))
	if err != nil {
		respondPayment(c, err)
		return
	}
	c.JSON(http.StatusOK, method)
}

// SavePaymentMethod updates whether a method is on, its receiving number and instructions.
func (h PaymentHandler) SavePaymentMethod(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req struct {
		Enabled       bool   `json:"enabled"`
		AccountNumber string `json:"account_number"`
		Instructions  string `json:"instructions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment settings"})
		return
	}
	err := h.Service.SaveMethod(c.Request.Context(), payments.Method{Code: c.Param("code"), Enabled: req.Enabled, AccountNumber: req.AccountNumber, Instructions: req.Instructions})
	respondPaymentReview(c, err)
}

// respondPaymentReview answers a successful owner action or maps its error.
func respondPaymentReview(c *gin.Context, err error) {
	c.Header("Cache-Control", "no-store")
	if err != nil {
		respondPayment(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"saved": true})
}

// respondPayment maps payment errors to safe messages; database details never reach the client.
func respondPayment(c *gin.Context, err error) {
	switch {
	case errors.Is(err, payments.ErrInvalidPayment):
		c.JSON(http.StatusBadRequest, gin.H{"error": "check the transaction ID, the bKash number (01XXXXXXXXX) and the amount"})
	case errors.Is(err, payments.ErrNotFound), errors.Is(err, orders.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, payments.ErrPriceNotFinal):
		c.JSON(http.StatusConflict, gin.H{"error": "the lens price is not confirmed yet"})
	case errors.Is(err, payments.ErrNotPayable):
		c.JSON(http.StatusConflict, gin.H{"error": "this order is not waiting for a payment"})
	case errors.Is(err, payments.ErrMethodOff):
		c.JSON(http.StatusConflict, gin.H{"error": "bKash payment is not available right now"})
	case errors.Is(err, payments.ErrTooManyPayments):
		c.JSON(http.StatusConflict, gin.H{"error": "too many payments on this order; please contact us"})
	case errors.Is(err, payments.ErrAlreadyReviewed):
		c.JSON(http.StatusConflict, gin.H{"error": "this payment was already reviewed"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not process the payment"})
	}
}
