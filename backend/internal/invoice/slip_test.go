package invoice

import (
	"strings"
	"testing"

	"bongkey/backend/internal/orders"
)

// TestReadableDate formats stored timestamps and keeps anything unexpected visible.
func TestReadableDate(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-07T12:15:23.347726+06:00": "7 Nov 2026, 12:15",
		"2026-11-07 12:15:23.3+06":         "7 Nov 2026, 12:15",
		"yesterday":                        "yesterday",
		"":                                 "",
	} {
		if got := readableDate(in); got != want {
			t.Fatalf("readableDate(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRenderSlip builds a PDF for a complete order and refuses an empty one.
func TestRenderSlip(t *testing.T) {
	order := orders.AdminDetail{Items: []orders.Item{{ProductName: "Noor Gold Rim", LensOption: "Frame only", Quantity: 1, TotalPrice: 2290}}}
	order.OrderNumber, order.Total = "ORD-000192", 2420
	pdf, err := RenderSlip(order, "https://shop.example")
	if err != nil || !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("slip not rendered: %v", err)
	}
	if _, err := RenderSlip(orders.AdminDetail{}, ""); err == nil {
		t.Fatal("empty order rendered a slip")
	}
}
