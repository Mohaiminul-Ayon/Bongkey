package orders

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	// ErrLensPriceNotFinal blocks the advance and confirmation while a lens price is still open.
	ErrLensPriceNotFinal = errors.New("lens price not confirmed")
	// ErrPrescriptionNotAccepted blocks confirmation until every prescription was checked.
	ErrPrescriptionNotAccepted = errors.New("prescription not accepted")
	// ErrAdvanceNotReceived blocks confirmation of a lens order until its advance is recorded.
	ErrAdvanceNotReceived = errors.New("advance not received")
	// ErrLensReviewClosed means the order is past the stage where this change is allowed.
	ErrLensReviewClosed = errors.New("order no longer accepts this lens change")
	// ErrInvalidLensReview means a review value is out of range.
	ErrInvalidLensReview = errors.New("invalid lens review")
)

// maxLensPrice bounds an owner-entered lens price; it matches the lens type limit.
const maxLensPrice = 200000

// maxPrescriptionNote matches the database limit on the owner's review note.
const maxPrescriptionNote = 500

// LensReviewStore applies one owner decision atomically and rechecks the order's stage while locked.
type LensReviewStore interface {
	SetPrescriptionStatus(ctx context.Context, orderID, itemID int, status, note string) error
	ReplacePrescription(ctx context.Context, orderID, itemID int, name string) (oldName string, err error)
	SetLensPrice(ctx context.Context, orderID, itemID, price int, customName string) error
	MarkAdvanceReceived(ctx context.Context, orderID int) error
	PrescriptionFile(ctx context.Context, orderID, itemID int) (string, error)
}

// LensReviewService lets the owner finish a lens order: check the prescription, confirm the price, record the advance.
// The order of these steps protects the customer: the price is settled before any money is taken.
type LensReviewService struct {
	Store LensReviewStore
	Files PrescriptionSaver
}

// SetPrescriptionStatus accepts a prescription or asks the customer for a new one, with the owner's private note.
func (s LensReviewService) SetPrescriptionStatus(ctx context.Context, orderID, itemID int, status, note string) error {
	note = strings.TrimSpace(note)
	if orderID <= 0 || itemID <= 0 || (status != "accepted" && status != "needs_new") || utf8.RuneCountInString(note) > maxPrescriptionNote {
		return ErrInvalidLensReview
	}
	return s.Store.SetPrescriptionStatus(ctx, orderID, itemID, status, note)
}

// AttachPrescription stores a file the customer sent outside the site (e.g. WhatsApp) on one lens line.
// The line goes back to "pending" so the owner accepts the new file explicitly. The old file is deleted only
// after the database points at the new one, and the new file is deleted if the change is refused.
func (s LensReviewService) AttachPrescription(ctx context.Context, orderID, itemID int, data []byte) error {
	if orderID <= 0 || itemID <= 0 {
		return ErrInvalidLensReview
	}
	name, err := s.Files.Save(data)
	if err != nil {
		return err
	}
	oldName, err := s.Store.ReplacePrescription(ctx, orderID, itemID, name)
	if err != nil {
		s.Files.Remove(name)
		return err
	}
	if oldName != "" {
		s.Files.Remove(oldName)
	}
	return nil
}

// SetLensPrice confirms the final lens price; customName describes a custom lens agreed in chat.
func (s LensReviewService) SetLensPrice(ctx context.Context, orderID, itemID, price int, customName string) error {
	customName = strings.TrimSpace(customName)
	if orderID <= 0 || itemID <= 0 || price < 0 || price > maxLensPrice || utf8.RuneCountInString(customName) > 120 {
		return ErrInvalidLensReview
	}
	return s.Store.SetLensPrice(ctx, orderID, itemID, price, customName)
}

// MarkAdvanceReceived records the advance collected outside the website, once every lens price is final.
func (s LensReviewService) MarkAdvanceReceived(ctx context.Context, orderID int) error {
	if orderID <= 0 {
		return ErrInvalidLensReview
	}
	return s.Store.MarkAdvanceReceived(ctx, orderID)
}

// PrescriptionFile returns the private stored name for one line's prescription.
func (s LensReviewService) PrescriptionFile(ctx context.Context, orderID, itemID int) (string, error) {
	if orderID <= 0 || itemID <= 0 {
		return "", ErrNotFound
	}
	return s.Store.PrescriptionFile(ctx, orderID, itemID)
}
