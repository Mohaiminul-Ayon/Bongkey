package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"bongkey/backend/internal/database"
	"bongkey/backend/internal/lens"
	"bongkey/backend/internal/orders"

	"github.com/gin-gonic/gin"
)

// maxOrderJSONBytes bounds the order details; files are bounded separately.
const maxOrderJSONBytes = 64 << 10

// maxCheckoutBodyBytes allows the order details plus the most prescription files one checkout accepts.
const maxCheckoutBodyBytes = maxOrderJSONBytes + 5*(lens.MaxPrescriptionBytes+4<<10)

type orderRequest struct {
	CustomerName        string             `json:"customer_name"`
	CustomerPhone       string             `json:"customer_phone"`
	CustomerEmail       string             `json:"customer_email"`
	EmailProof          string             `json:"email_proof"`
	ShippingAddress     string             `json:"shipping_address"`
	City                string             `json:"city"`
	DeliveryNote        string             `json:"delivery_note"`
	ExpectedDeliveryFee *int               `json:"expected_delivery_fee"`
	Items               []orderItemRequest `json:"items"`
}

type orderItemRequest struct {
	ProductID     int    `json:"product_id"`
	Quantity      int    `json:"quantity"`
	LensOption    string `json:"lens_option"`
	LensKind      string `json:"lens_kind"`
	LensPackageID int    `json:"lens_package_id"`
	LensPower     string `json:"lens_power"`
	SPHRight      *int   `json:"sph_right"`
	SPHLeft       *int   `json:"sph_left"`
	LensRequest   string `json:"lens_request"`
}

// CheckoutHandler maps order input and checkout decisions to HTTP.
type CheckoutHandler struct {
	Service orders.CheckoutService
}

// CreateOrder places an order. Frame-only carts send JSON; carts with prescriptions send multipart
// with the JSON in an "order" field and each file as "prescription_<item index>".
func (h CheckoutHandler) CreateOrder(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if !database.Ready() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not connected"})
		return
	}
	req, files, err := readOrderRequest(c)
	if errors.Is(err, lens.ErrInvalidPrescription) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "each prescription must be a clear PDF, JPG, PNG or HEIC file up to 5 MB"})
		return
	}
	if err != nil {
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
		items[i] = orders.CheckoutItem{
			ProductID: item.ProductID, Quantity: item.Quantity, LensOption: item.LensOption,
			LensKind: item.LensKind, LensPackageID: item.LensPackageID, LensPower: item.LensPower,
			SPHRight: item.SPHRight, SPHLeft: item.SPHLeft, LensRequest: item.LensRequest, Prescription: files[i],
		}
	}
	receipt, err := h.Service.Place(c.Request.Context(), orders.CheckoutRequest{
		IdempotencyKey: idempotencyKey,
		CustomerName:   req.CustomerName, CustomerPhone: req.CustomerPhone,
		CustomerEmail: req.CustomerEmail, EmailProof: req.EmailProof, ShippingAddress: req.ShippingAddress,
		City: req.City, DeliveryNote: req.DeliveryNote,
		ExpectedDeliveryFee: req.ExpectedDeliveryFee, Items: items,
	})
	if err != nil {
		switch {
		case errors.Is(err, orders.ErrInvalidCheckout):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order data"})
		case errors.Is(err, lens.ErrInvalidPrescription):
			c.JSON(http.StatusBadRequest, gin.H{"error": "each prescription must be a clear PDF, JPG, PNG or HEIC file up to 5 MB"})
		case errors.Is(err, orders.ErrUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "item unavailable or out of stock"})
		case errors.Is(err, orders.ErrLensUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "a lens type in your bag is no longer available for that frame; please choose again"})
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
		"total": receipt.Total, "payment_method": receipt.PaymentMethod,
		"order_status": "pending", "guest_access_token": receipt.GuestAccessToken,
	})
}

// readOrderRequest parses a bounded JSON or multipart checkout body; files[i] belongs to item i.
// File contents are checked here so a bad upload is refused before any database work.
func readOrderRequest(c *gin.Context) (orderRequest, [][]byte, error) {
	var req orderRequest
	if !strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOrderJSONBytes)
		if err := c.ShouldBindJSON(&req); err != nil {
			return req, nil, err
		}
		return req, make([][]byte, len(req.Items)), nil
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCheckoutBodyBytes)
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		return req, nil, err
	}
	defer c.Request.MultipartForm.RemoveAll()
	if err := json.Unmarshal([]byte(c.Request.FormValue("order")), &req); err != nil {
		return req, nil, err
	}
	files := make([][]byte, len(req.Items))
	for field, headers := range c.Request.MultipartForm.File {
		var index int
		if _, err := fmt.Sscanf(field, "prescription_%d", &index); err != nil || field != fmt.Sprintf("prescription_%d", index) ||
			index < 0 || index >= len(req.Items) || len(headers) != 1 {
			return req, nil, errors.New("unexpected upload field")
		}
		data, err := readUpload(headers[0])
		if err != nil {
			return req, nil, err
		}
		if _, err := lens.PrescriptionType(data); err != nil {
			return req, nil, err
		}
		files[index] = data
	}
	return req, files, nil
}

// readUpload reads one uploaded file, refusing anything above the prescription size limit.
func readUpload(header *multipart.FileHeader) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, lens.MaxPrescriptionBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > lens.MaxPrescriptionBytes {
		return nil, lens.ErrInvalidPrescription
	}
	return data, nil
}
