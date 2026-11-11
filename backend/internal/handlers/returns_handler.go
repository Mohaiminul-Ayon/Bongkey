package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bongkey/backend/internal/lens"
	"bongkey/backend/internal/orders"
	"bongkey/backend/internal/returns"
	"github.com/gin-gonic/gin"
)

// maxReturnBodyBytes allows the text fields plus the most photos one ticket accepts.
const maxReturnBodyBytes = 16<<10 + returns.MaxPhotos*(lens.MaxPrescriptionBytes+4<<10)

// ReturnsHandler takes return and exchange tickets from customers and lets staff work them.
// Customers prove access with the order secret (order page) or the order number and checkout phone (Track
// page), the same checks those pages already use; admin routes run after AdminAuth.
type ReturnsHandler struct {
	Service returns.Service
	Orders  orders.AccessService
}

// returnForm reads the multipart ticket: kind, message and up to MaxPhotos files named "photos".
// The caller must call RemoveAll on the parsed form.
func returnForm(c *gin.Context) (returns.Submission, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxReturnBodyBytes)
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		return returns.Submission{}, err
	}
	sub := returns.Submission{Kind: c.Request.FormValue("kind"), Message: c.Request.FormValue("message")}
	for field, headers := range c.Request.MultipartForm.File {
		if field != "photos" || len(headers) > returns.MaxPhotos {
			return returns.Submission{}, errors.New("unexpected upload field")
		}
		for _, header := range headers {
			data, err := readUpload(header)
			if err != nil {
				return returns.Submission{}, err
			}
			sub.Photos = append(sub.Photos, data)
		}
	}
	return sub, nil
}

// CreateFromOrderPage opens a ticket for the order on the customer's private order page.
func (h ReturnsHandler) CreateFromOrderPage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || token == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	order, err := h.Orders.Guest(c.Request.Context(), c.Param("id"), token)
	if errors.Is(err, orders.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load the order"})
		return
	}
	h.create(c, returns.Order{ID: order.ID, Status: order.Status, DeliveredAt: order.Tracking.DeliveredAt})
}

// CreateFromTrack opens a ticket after the same order number + phone check as the Track page, which is
// rate limited per client, so the form cannot be used to guess phones.
func (h ReturnsHandler) CreateFromTrack(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	sub, err := returnForm(c)
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		respondReturnUpload(c)
		return
	}
	view, err := h.Orders.Track(c.Request.Context(), c.Request.FormValue("order_number"), c.Request.FormValue("phone"), c.ClientIP())
	switch {
	case errors.Is(err, orders.ErrTrackLimited):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many tries; please wait 15 minutes"})
		return
	case errors.Is(err, orders.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no order matches this order number and phone"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load the order"})
		return
	}
	h.save(c, returns.Order{ID: view.ID, Status: view.Status, DeliveredAt: view.Tracking.DeliveredAt}, sub)
}

// create reads the form for an already authorized order and saves the ticket.
func (h ReturnsHandler) create(c *gin.Context, order returns.Order) {
	sub, err := returnForm(c)
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		respondReturnUpload(c)
		return
	}
	h.save(c, order, sub)
}

// save runs the use case and maps its rules to clear customer messages.
func (h ReturnsHandler) save(c *gin.Context, order returns.Order, sub returns.Submission) {
	ticket, err := h.Service.Create(c.Request.Context(), order, sub)
	switch {
	case errors.Is(err, returns.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "choose what you need and describe the problem in 5 to 1000 characters (up to 3 photos)"})
	case errors.Is(err, lens.ErrInvalidPrescription):
		c.JSON(http.StatusBadRequest, gin.H{"error": "each photo must be a clear JPG, PNG or HEIC image of 5 MB or less"})
	case errors.Is(err, returns.ErrNotDelivered):
		c.JSON(http.StatusConflict, gin.H{"error": "you can ask for a return or exchange after the order is delivered"})
	case errors.Is(err, returns.ErrWindowClosed):
		c.JSON(http.StatusConflict, gin.H{"error": "the time for this request is over (exchange: 3 days, problem: 15 days after delivery); please message us on WhatsApp"})
	case errors.Is(err, returns.ErrOpenTicket):
		c.JSON(http.StatusConflict, gin.H{"error": "this order already has an open request; we will contact you about it"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not send your request"})
	default:
		c.JSON(http.StatusCreated, gin.H{"id": ticket.ID})
	}
}

// respondReturnUpload answers a form that could not be read: too large, an extra field or a broken file.
func respondReturnUpload(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "the request could not be read; send up to 3 photos of 5 MB or less"})
}

// AdminList returns tickets for staff, optionally one status (?status=open).
func (h ReturnsHandler) AdminList(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	list, err := h.Service.List(c.Request.Context(), c.Query("status"))
	if err != nil {
		respondReturnAdmin(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, list)
}

// Update moves a ticket along or closes it as exchanged, replaced or declined (with a note).
func (h ReturnsHandler) Update(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, ok := ticketID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req struct {
		Status  string `json:"status"`
		Outcome string `json:"outcome"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid update"})
		return
	}
	respondReturnAdmin(c, h.Service.Update(c.Request.Context(), id, req.Status, req.Outcome, req.Note), gin.H{"saved": true})
}

// RecordRefund saves money actually sent back and closes the ticket as refunded.
func (h ReturnsHandler) RecordRefund(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, ok := ticketID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req struct {
		Amount    int    `json:"amount"`
		Method    string `json:"method"`
		Reference string `json:"reference"`
		// PaidOn is the day the money was sent (YYYY-MM-DD, Bangladesh time); empty means now.
		PaidOn string `json:"paid_on"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid refund"})
		return
	}
	var paidAt time.Time
	if req.PaidOn != "" {
		day, err := time.ParseInLocation("2006-01-02", req.PaidOn, time.FixedZone("Asia/Dhaka", 6*60*60))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid refund date"})
			return
		}
		paidAt = day
	}
	err := h.Service.RecordRefund(c.Request.Context(), id, returns.Refund{Amount: req.Amount, Method: req.Method, Reference: req.Reference, PaidAt: paidAt}, req.Note)
	respondReturnAdmin(c, err, gin.H{"saved": true})
}

// Photo sends one ticket photo to staff from the private folder, never through a public URL.
func (h ReturnsHandler) Photo(c *gin.Context) {
	id, ok := ticketID(c)
	if !ok {
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "photo not found"})
		return
	}
	path, err := h.Service.PhotoPath(c.Request.Context(), id, index)
	if err != nil {
		respondReturnAdmin(c, err, nil)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "photo file is missing"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	c.Header("Content-Disposition", "inline")
	c.Data(http.StatusOK, prescriptionContentTypes[filepath.Ext(path)], data)
}

// ticketID reads the :id path value; it answers 404 itself when it is not a number.
func ticketID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return 0, false
	}
	return id, true
}

// respondReturnAdmin maps staff-side errors to safe messages, or sends ok when there is no error.
func respondReturnAdmin(c *gin.Context, err error, ok gin.H) {
	switch {
	case err == nil:
		c.JSON(http.StatusOK, ok)
	case errors.Is(err, returns.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "check the fields: closing needs an outcome, declining needs a note, a bKash refund needs its TrxID, and a refund cannot be more than the order total or dated in the future"})
	case errors.Is(err, returns.ErrNotRefundable):
		c.JSON(http.StatusConflict, gin.H{"error": "only an open defect or wrong-item request can be refunded; an exchange is settled by swapping the frame"})
	case errors.Is(err, returns.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found or already closed"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update the request"})
	}
}
