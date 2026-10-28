package invoice

import (
	_ "embed"
	"fmt"
	"strings"

	"bongkey/backend/internal/orders"
	"github.com/signintech/gopdf"
)

//go:embed fonts/HindSiliguri-Regular.ttf
var invoiceFont []byte

// Render builds a private invoice from accepted order and item snapshots.
func Render(order orders.AdminDetail) ([]byte, error) {
	if order.InvoiceNumber == "" || len(order.Items) == 0 {
		return nil, fmt.Errorf("invoice snapshot incomplete")
	}
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()
	if err := pdf.AddTTFFontData("Bongkey", invoiceFont); err != nil {
		return nil, fmt.Errorf("load invoice font: %w", err)
	}
	layout := pdfLayout{pdf: pdf, y: 42}
	for _, entry := range []struct {
		text string
		size int
	}{
		{"BONGKEY", 23}, {"INVOICE", 15},
		{"Invoice: " + order.InvoiceNumber, 11}, {"Order: " + order.OrderNumber, 11},
		{"Order date: " + order.CreatedAt, 10}, {"Invoice issued: " + order.InvoiceIssuedAt, 10},
		{"Customer: " + order.CustomerName, 11}, {"Email: " + order.CustomerEmail, 10},
		{"Phone: " + order.CustomerPhone, 10},
		{"Deliver to: " + order.ShippingAddress + ", " + order.City, 10},
		{"Items", 13},
	} {
		if err := layout.line(entry.text, entry.size); err != nil {
			return nil, err
		}
	}
	for _, item := range order.Items {
		if err := layout.line(fmt.Sprintf("%s | SKU %s | %d x BDT %d = BDT %d", item.ProductName, item.SKU, item.Quantity, item.UnitPrice, item.TotalPrice), 10); err != nil {
			return nil, err
		}
		// Lens selections currently have no separately priced line in the accepted order.
		if item.LensOption != "" && item.LensOption != "Frame only" {
			if err := layout.line("Lens selection: "+item.LensOption+" (no separate charge)", 9); err != nil {
				return nil, err
			}
		}
	}
	for _, entry := range []string{
		fmt.Sprintf("Subtotal: BDT %d", order.Subtotal),
		"Discount: BDT 0",
		fmt.Sprintf("Delivery: BDT %d", order.DeliveryFee),
		fmt.Sprintf("Total: BDT %d", order.Total),
		"Payment method: " + order.PaymentMethod,
		"Payment status: " + order.PaymentStatus,
	} {
		if err := layout.line(entry, 11); err != nil {
			return nil, err
		}
	}
	// Only unpaid COD is supported; other states must not invent a collected amount.
	if order.PaymentStatus == "unpaid" {
		if err := layout.line(fmt.Sprintf("Paid: BDT 0 | Balance: BDT %d", order.Total), 11); err != nil {
			return nil, err
		}
	} else if err := layout.line("Paid / balance: consult payment record", 11); err != nil {
		return nil, err
	}
	return pdf.GetBytesPdfReturnErr()
}

// pdfLayout tracks page height so long addresses and item lists continue safely.
type pdfLayout struct {
	pdf *gopdf.GoPdf
	y   float64
}

// line wraps one value and adds a page before any line reaches the foot margin.
func (l *pdfLayout) line(value string, size int) error {
	if err := l.pdf.SetFont("Bongkey", "", size); err != nil {
		return err
	}
	value = strings.Join(strings.Fields(value), " ")
	parts, err := l.pdf.SplitText(value, 510)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		parts = []string{" "}
	}
	for _, part := range parts {
		if l.y > 790 {
			l.pdf.AddPage()
			l.y = 42
		}
		l.pdf.SetXY(42, l.y)
		if err := l.pdf.Cell(nil, part); err != nil {
			return err
		}
		l.y += float64(size) + 8
	}
	return nil
}
