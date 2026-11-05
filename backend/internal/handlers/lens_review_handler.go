package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"bongkey/backend/internal/lens"
	"bongkey/backend/internal/orders"
	"github.com/gin-gonic/gin"
)

// prescriptionContentTypes maps stored extensions to the type sent to the admin browser.
var prescriptionContentTypes = map[string]string{
	".pdf": "application/pdf", ".jpg": "image/jpeg", ".png": "image/png", ".heic": "image/heic",
}

// LensReviewHandler maps the owner's lens-order decisions to the review use case; every route runs after AdminAuth.
type LensReviewHandler struct {
	Service orders.LensReviewService
	Files   lens.PrescriptionStore
}

// SetPrescriptionStatus records "accepted" or "needs_new" for one line's prescription, with an optional private note.
func (h LensReviewHandler) SetPrescriptionStatus(c *gin.Context) {
	orderID, itemID, ok := orderItemIDs(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}
	respondLensReview(c, h.Service.SetPrescriptionStatus(c.Request.Context(), orderID, itemID, req.Status, req.Note))
}

// AttachPrescription stores one file the customer sent outside the site, sent as the multipart field "file".
func (h LensReviewHandler) AttachPrescription(c *gin.Context) {
	orderID, itemID, ok := orderItemIDs(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, lens.MaxPrescriptionBytes+8<<10)
	header, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "choose a PDF or photo up to 5 MB"})
		return
	}
	data, err := readUpload(header)
	if err == nil {
		_, err = lens.PrescriptionType(data)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the file must be a PDF, JPEG, PNG or HEIC up to 5 MB"})
		return
	}
	respondLensReview(c, h.Service.AttachPrescription(c.Request.Context(), orderID, itemID, data))
}

// SetLensPrice confirms one line's final lens price, and the custom lens description when given.
func (h LensReviewHandler) SetLensPrice(c *gin.Context) {
	orderID, itemID, ok := orderItemIDs(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<10)
	var req struct {
		Price *int   `json:"price"`
		Name  string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Price == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a lens price is required"})
		return
	}
	respondLensReview(c, h.Service.SetLensPrice(c.Request.Context(), orderID, itemID, *req.Price, req.Name))
}

// MarkAdvanceReceived records that the moderator collected the lens order's advance.
func (h LensReviewHandler) MarkAdvanceReceived(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}
	respondLensReview(c, h.Service.MarkAdvanceReceived(c.Request.Context(), orderID))
}

// Prescription streams one private prescription file to the admin; nothing is cached and the browser may not guess its type.
func (h LensReviewHandler) Prescription(c *gin.Context) {
	orderID, itemID, ok := orderItemIDs(c)
	if !ok {
		return
	}
	name, err := h.Service.PrescriptionFile(c.Request.Context(), orderID, itemID)
	if errors.Is(err, orders.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "prescription not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load prescription"})
		return
	}
	path, err := h.Files.Path(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "prescription not found"})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "prescription file is missing"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	c.Header("Content-Disposition", "inline")
	c.Data(http.StatusOK, prescriptionContentTypes[filepath.Ext(name)], data)
}

// orderItemIDs parses both path IDs and answers 400 itself when either is invalid.
func orderItemIDs(c *gin.Context) (int, int, bool) {
	orderID, err := strconv.Atoi(c.Param("id"))
	itemID, itemErr := strconv.Atoi(c.Param("itemId"))
	if err != nil || itemErr != nil || orderID <= 0 || itemID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order or item id"})
		return 0, 0, false
	}
	return orderID, itemID, true
}

// respondLensReview maps review results to safe owner messages.
func respondLensReview(c *gin.Context, err error) {
	c.Header("Cache-Control", "no-store")
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"saved": true})
	case errors.Is(err, orders.ErrInvalidLensReview):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid value"})
	case errors.Is(err, orders.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "order line not found"})
	case errors.Is(err, orders.ErrLensPriceNotFinal):
		c.JSON(http.StatusConflict, gin.H{"error": "confirm every lens price before recording the advance"})
	case errors.Is(err, orders.ErrLensReviewClosed):
		c.JSON(http.StatusConflict, gin.H{"error": "this order is past the stage for this change"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save"})
	}
}
