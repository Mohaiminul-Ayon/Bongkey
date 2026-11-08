package invoice

import (
	"fmt"
	"time"

	"bongkey/backend/internal/orders"
	"github.com/signintech/gopdf"
)

// RenderSlip builds the order slip a customer downloads right after checkout: order number, products, total and
// how to track. It deliberately says it is not an invoice, because the owner has not confirmed the order yet.
func RenderSlip(order orders.AdminDetail, siteURL string) ([]byte, error) {
	if order.OrderNumber == "" || len(order.Items) == 0 {
		return nil, fmt.Errorf("order slip snapshot incomplete")
	}
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()
	if err := pdf.AddTTFFontData("Bongkey", invoiceFont); err != nil {
		return nil, fmt.Errorf("load slip font: %w", err)
	}
	layout := pdfLayout{pdf: pdf, y: 42}
	lines := []struct {
		text string
		size int
	}{
		{"BONGKEY", 23}, {"ORDER SLIP", 15},
		{"This is not an invoice. We call you to confirm the order; your invoice comes after that.", 10},
		{"Order: " + order.OrderNumber, 13}, {"Order date: " + readableDate(order.CreatedAt), 10}, {"Items", 13},
	}
	for _, item := range order.Items {
		lines = append(lines, struct {
			text string
			size int
		}{fmt.Sprintf("%s (%s) x%d = BDT %d", item.ProductName, item.LensOption, item.Quantity, item.TotalPrice), 10})
	}
	for _, entry := range []string{
		fmt.Sprintf("Subtotal: BDT %d", order.Subtotal),
		fmt.Sprintf("Delivery: BDT %d", order.DeliveryFee),
		fmt.Sprintf("Total: BDT %d", order.Total),
		"Payment: " + label(paymentMethodLabel, order.PaymentMethod),
	} {
		lines = append(lines, struct {
			text string
			size int
		}{entry, 11})
	}
	if due := order.AdvanceRequired - order.AmountPaid; due > 0 {
		lines = append(lines, struct {
			text string
			size int
		}{fmt.Sprintf("Pay by bKash before we confirm: BDT %d", due), 11})
	}
	lines = append(lines, struct {
		text string
		size int
	}{"Track your order at " + siteURL + "/track with this order number and your phone number.", 11})
	for _, entry := range lines {
		if err := layout.line(entry.text, entry.size); err != nil {
			return nil, err
		}
	}
	return pdf.GetBytesPdfReturnErr()
}

// readableDate turns a stored timestamp (the driver's 2026-11-07T12:15:23+06:00, or PostgreSQL's text form
// 2026-11-07 12:15:23.3+06) into "7 Nov 2026, 12:15"; an unexpected format is printed as stored rather than hidden.
func readableDate(value string) string {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999-07"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("2 Jan 2006, 15:04")
		}
	}
	return value
}
