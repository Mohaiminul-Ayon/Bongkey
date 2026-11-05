package orders

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"bongkey/backend/internal/delivery"
	"bongkey/backend/internal/security"
)

const maxOrderAmount = 1<<31 - 1

// frameOnlyLens labels lines without lenses.
const frameOnlyLens = "Frame only"

// Lens kinds and power choices accepted on an order line.
const (
	LensKindPackage = "package"
	LensKindCustom  = "custom"
	// maxLensRequest matches the database limit on a custom lens description.
	maxLensRequest = 500
	LensNoPower    = "no_power"
	LensWithPower  = "with_power"
)

// maxPrescriptionLines bounds how many files one checkout may upload.
const maxPrescriptionLines = 5

var (
	// ErrInvalidCheckout means the submitted order cannot be accepted as written.
	ErrInvalidCheckout = errors.New("invalid checkout")
	// ErrUnavailable hides whether a product is missing, inactive, or out of stock.
	ErrUnavailable = errors.New("product unavailable")
	// ErrIdempotencyConflict means one retry key was reused for different order data.
	ErrIdempotencyConflict = errors.New("idempotency key reused with different checkout")
	// ErrDeliveryUnavailable means the configured delivery charge could not be read safely.
	ErrDeliveryUnavailable = errors.New("delivery fee unavailable")
	// ErrDeliveryFeeChanged requires customer consent to a changed delivery charge.
	ErrDeliveryFeeChanged = errors.New("delivery fee changed")
	// ErrLensUnavailable means the lens type is disabled or does not fit the chosen frame.
	ErrLensUnavailable = errors.New("lens type unavailable for this frame")
	// ErrPaymentUnavailable means the customer chose an online payment the owner has turned off.
	ErrPaymentUnavailable = errors.New("payment method unavailable")
)

// Checkout payment choices. An empty choice keeps the old default: cash on delivery, or an advance for lenses.
const (
	PayCashOnDelivery = "cash_on_delivery"
	PayBkash          = "bkash"
)

// PaymentOptions tells checkout whether an online payment method is turned on.
type PaymentOptions interface {
	Enabled(ctx context.Context, code string) (bool, error)
}

// CheckoutItem is one requested product and lens choice.
// Empty lens fields are left out of the retry fingerprint, so frame-only retries still match orders placed before lenses existed.
type CheckoutItem struct {
	ProductID     int
	Quantity      int
	LensOption    string
	LensKind      string `json:",omitempty"`
	LensPackageID int    `json:",omitempty"`
	LensPower     string `json:",omitempty"`
	// SPH values are hundredths of a diopter (-150 = -1.50); nil means the customer is not sure.
	SPHRight *int `json:",omitempty"`
	SPHLeft  *int `json:",omitempty"`
	// LensRequest is the customer's own description of a custom lens; required for custom, empty otherwise.
	LensRequest string `json:",omitempty"`
	// Prescription is the uploaded file; it is stored privately and never fingerprinted.
	Prescription []byte `json:"-"`
}

// LensChoice is the server's current price and rules for one lens type on one frame.
type LensChoice struct {
	Name                 string
	Price                int
	MaxPower             int
	PrescriptionRequired bool
}

// OrderLine is the priced snapshot of one accepted item.
type OrderLine struct {
	LensName           string
	LensRequest        string
	LensPrice          int
	LensPriceFinal     bool
	PrescriptionFile   string
	PrescriptionStatus string
	LineTotal          int
}

// PrescriptionSaver stores uploaded prescriptions privately; Remove undoes a save whose order did not commit.
type PrescriptionSaver interface {
	Save([]byte) (string, error)
	Remove(string)
}

// CheckoutRequest contains the customer details and items needed to place a COD order.
type CheckoutRequest struct {
	IdempotencyKey      string
	CustomerName        string
	CustomerPhone       string
	CustomerEmail       string
	EmailProof          string
	EmailVerified       bool
	ShippingAddress     string
	City                string
	DeliveryNote        string
	ExpectedDeliveryFee *int
	// PaymentMethod is the customer's choice: "", PayCashOnDelivery or PayBkash.
	PaymentMethod string
	Items         []CheckoutItem
}

// CheckoutReceipt contains the accepted order totals and one-time guest token.
type CheckoutReceipt struct {
	ID               int
	OrderNumber      string
	InvoiceNumber    string
	Subtotal         int
	DeliveryFee      int
	Total            int
	GuestAccessToken string
	PaymentMethod    string
}

// ReservedProduct contains the server-authoritative name and price of a reserved product.
type ReservedProduct struct {
	Name  string
	Price int
	SKU   string
}

// CheckoutTx persists all pieces of an order inside one transaction.
type CheckoutTx interface {
	FindIdempotentOrder(context.Context, string) (CheckoutReceipt, string, bool, error)
	ReserveProduct(context.Context, int, int) (ReservedProduct, error)
	LensForProduct(ctx context.Context, packageID, productID int) (LensChoice, error)
	InsertOrder(ctx context.Context, request CheckoutRequest, subtotal, deliveryFee, total int, tokenHash, keyHash, requestHash, paymentMethod, paymentStatus string) (int, string, error)
	InsertItem(context.Context, int, CheckoutItem, ReservedProduct, OrderLine) error
	InsertInvoice(context.Context, int, CheckoutRequest, int, int, int) (string, error)
}

// CheckoutStore supplies one atomic database transaction for a checkout.
type CheckoutStore interface {
	InTransaction(context.Context, func(CheckoutTx) error) error
}

// DeliveryFeeProvider supplies the server-authoritative delivery charge.
type DeliveryFeeProvider interface {
	DeliveryFee(context.Context, string) (int, error)
}

// CheckoutService enforces verified email, retry, stock, and total rules.
type CheckoutService struct {
	Store               CheckoutStore
	EmailProofSecret    string
	OrderTokenSecret    string
	DeliveryFeeProvider DeliveryFeeProvider
	Prescriptions       PrescriptionSaver
	// PaymentOptions may be nil; then no online payment can be chosen for a frame-only order.
	PaymentOptions PaymentOptions
}

// Place reserves products in a stable order and commits the order and invoice together.
func (s CheckoutService) Place(ctx context.Context, request CheckoutRequest) (CheckoutReceipt, error) {
	if !ValidIdempotencyKey(request.IdempotencyKey) || request.CustomerName == "" || request.CustomerPhone == "" || request.ShippingAddress == "" || request.City == "" || len(request.Items) == 0 {
		return CheckoutReceipt{}, ErrInvalidCheckout
	}
	request.CustomerEmail = strings.ToLower(strings.TrimSpace(request.CustomerEmail))
	request.EmailVerified = request.CustomerEmail != ""
	quantities := make(map[int]int, len(request.Items))
	hasLens, prescriptions := false, 0
	for _, item := range request.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 || item.Quantity > maxOrderAmount-quantities[item.ProductID] || !validLensRequest(item) {
			return CheckoutReceipt{}, ErrInvalidCheckout
		}
		quantities[item.ProductID] += item.Quantity
		hasLens = hasLens || item.LensKind != ""
		if item.Prescription != nil {
			prescriptions++
		}
	}
	if prescriptions > maxPrescriptionLines {
		return CheckoutReceipt{}, ErrInvalidCheckout
	}
	paymentMethod, paymentStatus, err := s.paymentFor(ctx, request.PaymentMethod, hasLens)
	if err != nil {
		return CheckoutReceipt{}, err
	}
	productIDs := make([]int, 0, len(quantities))
	for id := range quantities {
		productIDs = append(productIDs, id)
	}
	sort.Ints(productIDs)

	requestHash, err := checkoutRequestHash(request)
	if err != nil {
		return CheckoutReceipt{}, err
	}
	idempotencyKeyHash := textHash(request.IdempotencyKey)
	guestToken, tokenHash, err := security.IdempotentOrderToken(s.OrderTokenSecret, request.IdempotencyKey)
	if err != nil {
		return CheckoutReceipt{}, err
	}
	var receipt CheckoutReceipt
	var savedFiles []string
	err = s.Store.InTransaction(ctx, func(tx CheckoutTx) error {
		existing, storedRequestHash, found, err := tx.FindIdempotentOrder(ctx, idempotencyKeyHash)
		if err != nil {
			return err
		}
		if found {
			if subtle.ConstantTimeCompare([]byte(storedRequestHash), []byte(requestHash)) != 1 {
				return ErrIdempotencyConflict
			}
			receipt = existing
			return nil
		}
		for _, item := range request.Items {
			if item.LensKind == "" && item.LensOption != "" && item.LensOption != frameOnlyLens {
				return ErrInvalidCheckout
			}
		}
		if request.ExpectedDeliveryFee == nil || *request.ExpectedDeliveryFee < 0 {
			return ErrInvalidCheckout
		}
		// Existing guest orders may retry their accepted key; new orders require verified email.
		if !request.EmailVerified {
			return ErrInvalidCheckout
		}
		verifiedEmail, err := security.VerifyEmailProof(s.EmailProofSecret, request.EmailProof)
		if err != nil || verifiedEmail != request.CustomerEmail {
			return ErrInvalidCheckout
		}
		if s.DeliveryFeeProvider == nil {
			return ErrDeliveryUnavailable
		}
		deliveryFee, err := s.DeliveryFeeProvider.DeliveryFee(ctx, request.City)
		if errors.Is(err, delivery.ErrInvalidDistrict) {
			return ErrInvalidCheckout
		}
		if err != nil || deliveryFee < 0 || deliveryFee > maxOrderAmount {
			return ErrDeliveryUnavailable
		}
		if deliveryFee != *request.ExpectedDeliveryFee {
			return ErrDeliveryFeeChanged
		}

		products := make(map[int]ReservedProduct, len(productIDs))
		for _, id := range productIDs {
			product, err := tx.ReserveProduct(ctx, id, quantities[id])
			if err != nil {
				return err
			}
			if product.Price <= 0 {
				return ErrUnavailable
			}
			products[id] = product
		}

		lines := make([]OrderLine, len(request.Items))
		subtotal := 0
		for i, item := range request.Items {
			line := OrderLine{}
			if item.LensKind == LensKindPackage {
				choice, err := tx.LensForProduct(ctx, item.LensPackageID, item.ProductID)
				if err != nil {
					return err
				}
				if choice.PrescriptionRequired && item.LensPower != LensWithPower {
					return ErrInvalidCheckout
				}
				line = priceLensLine(item, choice)
			} else if item.LensKind == LensKindCustom {
				line = OrderLine{LensName: "Custom lens", LensRequest: strings.TrimSpace(item.LensRequest), PrescriptionStatus: "pending"}
			}
			lineTotal := (int64(products[item.ProductID].Price) + int64(line.LensPrice)) * int64(item.Quantity)
			if lineTotal > int64(maxOrderAmount)-int64(deliveryFee)-int64(subtotal) {
				return ErrInvalidCheckout
			}
			line.LineTotal = int(lineTotal)
			lines[i] = line
			subtotal += int(lineTotal)
		}
		total := subtotal + deliveryFee
		orderID, orderNumber, err := tx.InsertOrder(ctx, request, subtotal, deliveryFee, total, tokenHash, idempotencyKeyHash, requestHash, paymentMethod, paymentStatus)
		if err != nil {
			return err
		}
		for i, item := range request.Items {
			item.LensOption = lensLabel(item, lines[i])
			// Files are written only after every price and stock rule passed; a later failure removes them below.
			if item.Prescription != nil {
				if s.Prescriptions == nil {
					return ErrInvalidCheckout
				}
				name, err := s.Prescriptions.Save(item.Prescription)
				if err != nil {
					return err
				}
				savedFiles = append(savedFiles, name)
				lines[i].PrescriptionFile = name
			}
			if err := tx.InsertItem(ctx, orderID, item, products[item.ProductID], lines[i]); err != nil {
				return err
			}
		}
		invoiceNumber, err := tx.InsertInvoice(ctx, orderID, request, subtotal, deliveryFee, total)
		if err != nil {
			return err
		}
		receipt = CheckoutReceipt{ID: orderID, OrderNumber: orderNumber, InvoiceNumber: invoiceNumber, Subtotal: subtotal, DeliveryFee: deliveryFee, Total: total, PaymentMethod: paymentMethod}
		return nil
	})
	if err != nil {
		for _, name := range savedFiles {
			s.Prescriptions.Remove(name)
		}
		return CheckoutReceipt{}, err
	}
	receipt.GuestAccessToken = guestToken
	return receipt, nil
}

// validLensRequest checks the shape of one line's lens choice before any database work.
// A lens line may hold several identical pairs; they share the line's one prescription.
func validLensRequest(item CheckoutItem) bool {
	switch item.LensKind {
	case "":
		// The frame-only label is checked after the retry lookup, so accepted legacy retries keep their receipt.
		return item.LensPackageID == 0 && item.LensPower == "" && item.SPHRight == nil && item.SPHLeft == nil && item.Prescription == nil && item.LensRequest == ""
	case LensKindPackage:
		if item.LensPackageID <= 0 || item.LensOption != "" || item.LensRequest != "" {
			return false
		}
		if item.LensPower == LensNoPower {
			return item.SPHRight == nil && item.SPHLeft == nil && item.Prescription == nil
		}
		return item.LensPower == LensWithPower && item.Prescription != nil && validSPH(item.SPHRight) && validSPH(item.SPHLeft)
	case LensKindCustom:
		// The customer describes the lens; powers and price are agreed by phone or chat, and a prescription file is optional.
		request := strings.TrimSpace(item.LensRequest)
		return request != "" && utf8.RuneCountInString(request) <= maxLensRequest &&
			item.LensPackageID == 0 && item.LensPower == "" && item.LensOption == "" && item.SPHRight == nil && item.SPHLeft == nil
	default:
		return false
	}
}

// validSPH accepts "not sure" or a power from -20.00 to +12.00 in 0.25 steps.
func validSPH(sph *int) bool {
	return sph == nil || (*sph >= -2000 && *sph <= 1200 && *sph%25 == 0)
}

// priceLensLine applies the fixed price; it is final only when the stated powers are known and within the type's limit.
// Otherwise the owner confirms the price after checking the prescription, before taking the advance.
func priceLensLine(item CheckoutItem, choice LensChoice) OrderLine {
	line := OrderLine{LensName: choice.Name, LensPrice: choice.Price, LensPriceFinal: true}
	if item.LensPower == LensWithPower {
		line.PrescriptionStatus = "pending"
		line.LensPriceFinal = withinPower(item.SPHRight, choice.MaxPower) && withinPower(item.SPHLeft, choice.MaxPower)
	}
	return line
}

// withinPower is false for "not sure", because an unknown power cannot be priced safely.
func withinPower(sph *int, maxPower int) bool {
	return sph != nil && *sph <= maxPower && *sph >= -maxPower
}

// lensLabel is the human-readable lens text saved on the line and shown on the invoice.
func lensLabel(item CheckoutItem, line OrderLine) string {
	switch item.LensKind {
	case LensKindPackage:
		if item.LensPower == LensWithPower {
			return line.LensName + " · with power"
		}
		return line.LensName + " · no power"
	case LensKindCustom:
		return "Custom lens · price confirmed by call"
	default:
		return frameOnlyLens
	}
}

// ValidIdempotencyKey accepts opaque browser-generated retry keys with bounded safe characters.
func ValidIdempotencyKey(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	for _, char := range key {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' && char != ':' {
			return false
		}
	}
	return true
}

// paymentFor turns the customer's choice into the saved payment method and status.
// Lenses are cut to one customer's prescription, so lens orders always pay an advance and never cash on delivery;
// their advance may still be collected by phone, so bKash being off does not block them.
// A frame-only order may choose bKash only while the owner has it turned on, and is then paid in full before shipping.
func (s CheckoutService) paymentFor(ctx context.Context, choice string, hasLens bool) (string, string, error) {
	switch {
	case choice != "" && choice != PayCashOnDelivery && choice != PayBkash:
		return "", "", ErrInvalidCheckout
	case hasLens && choice == PayCashOnDelivery:
		return "", "", ErrInvalidCheckout
	case hasLens:
		return "advance", "advance_due", nil
	case choice != PayBkash:
		return "cash_on_delivery", "unpaid", nil
	case s.PaymentOptions == nil:
		return "", "", ErrPaymentUnavailable
	}
	enabled, err := s.PaymentOptions.Enabled(ctx, PayBkash)
	if err != nil {
		return "", "", err
	}
	if !enabled {
		return "", "", ErrPaymentUnavailable
	}
	return "advance", "advance_due", nil
}

// checkoutRequestHash excludes proof and expected fee so accepted retries retain the original receipt after either changes.
func checkoutRequestHash(request CheckoutRequest) (string, error) {
	// PrescriptionFileURL stays empty in the fingerprint so retries of orders accepted before it was removed still match.
	payload := struct {
		CustomerName, CustomerPhone, CustomerEmail, ShippingAddress, City, DeliveryNote, PrescriptionFileURL string
		EmailVerified                                                                                        bool
		Items                                                                                                []CheckoutItem
		// PaymentMethod is left out when empty so retries of orders placed before the choice existed still match.
		PaymentMethod string `json:",omitempty"`
	}{
		CustomerName: request.CustomerName, CustomerPhone: request.CustomerPhone, CustomerEmail: request.CustomerEmail,
		ShippingAddress: request.ShippingAddress, City: request.City, DeliveryNote: request.DeliveryNote,
		EmailVerified: request.EmailVerified, Items: request.Items, PaymentMethod: request.PaymentMethod,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return textHash(string(encoded)), nil
}

// textHash returns a stable non-reversible database key for checkout coordination.
func textHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
