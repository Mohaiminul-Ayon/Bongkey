package notifications

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"bongkey/backend/internal/callbacks"
	"bongkey/backend/internal/mail"
	"bongkey/backend/internal/orders"
	"bongkey/backend/internal/returns"
)

// memoryStore is an in-memory queue whose unique keys behave like the database's.
type memoryStore struct {
	rows     map[string]Outgoing
	payments map[int]PaymentNotice
	marks    []string
	summary  DailySummary
}

func newMemoryStore() *memoryStore {
	return &memoryStore{rows: map[string]Outgoing{}, payments: map[int]PaymentNotice{}}
}

// Enqueue keeps the first row per key, like ON CONFLICT DO NOTHING.
func (s *memoryStore) Enqueue(_ context.Context, out Outgoing) error {
	if _, ok := s.rows[out.Key]; !ok {
		s.rows[out.Key] = out
	}
	return nil
}

// HasEvent reports a queued key.
func (s *memoryStore) HasEvent(_ context.Context, key string) (bool, error) {
	_, ok := s.rows[key]
	return ok, nil
}

// Payment returns a stored payment notice.
func (s *memoryStore) Payment(_ context.Context, id int) (PaymentNotice, error) {
	p, ok := s.payments[id]
	if !ok {
		return PaymentNotice{}, errors.New("payment not found")
	}
	return p, nil
}

// ClaimDue is unused by these tests; worker tests call send directly.
func (s *memoryStore) ClaimDue(context.Context, int) ([]Queued, error) { return nil, nil }

// MarkSent records the outcome for assertions.
func (s *memoryStore) MarkSent(_ context.Context, id int64) error {
	s.marks = append(s.marks, "sent")
	return nil
}

// MarkSkipped records the outcome for assertions.
func (s *memoryStore) MarkSkipped(_ context.Context, id int64) error {
	s.marks = append(s.marks, "skipped")
	return nil
}

// MarkRetry records the delay for assertions.
func (s *memoryStore) MarkRetry(_ context.Context, id int64, next time.Time, _ string) error {
	s.marks = append(s.marks, "retry "+next.Format("15:04"))
	return nil
}

// MarkFailed records the outcome for assertions.
func (s *memoryStore) MarkFailed(_ context.Context, id int64, _ string) error {
	s.marks = append(s.marks, "failed")
	return nil
}

// Summary returns the prepared summary.
func (s *memoryStore) Summary(context.Context, time.Time, time.Time) (DailySummary, error) {
	return s.summary, nil
}

// fixedOrders returns one prepared order.
type fixedOrders struct{ order orders.AdminDetail }

// Admin returns the prepared order for its ID.
func (f *fixedOrders) Admin(_ context.Context, id string) (orders.AdminDetail, error) {
	if id != strconv.Itoa(f.order.ID) {
		return orders.AdminDetail{}, orders.ErrNotFound
	}
	return f.order, nil
}

// sampleOrder is a frame order with private details that the order-placed email must not show.
func sampleOrder() orders.AdminDetail {
	order := orders.AdminDetail{CustomerEmail: "buyer@example.com", PaymentMethod: "cash_on_delivery", Items: []orders.Item{{ProductName: "Noor Gold Rim", LensOption: "Frame only", Quantity: 1, TotalPrice: 2290}}}
	order.ID, order.OrderNumber, order.InvoiceNumber, order.CustomerName = 160, "ORD-000160", "INV-000106", "Rahim Uddin"
	order.CustomerPhone, order.ShippingAddress, order.City = "01712345678", "House 7, Road 3, Agrabad", "Chattogram"
	order.Subtotal, order.DeliveryFee, order.Total, order.AdvanceRequired, order.Status = 2290, 130, 2420, 130, "pending"
	return order
}

// TestOrderPlacedEmailHidesPrivateDetails sends only what a stranger with a mistyped email may see, once.
func TestOrderPlacedEmailHidesPrivateDetails(t *testing.T) {
	store := newMemoryStore()
	service := Service{Store: store, Orders: &fixedOrders{sampleOrder()}, SiteURL: "https://shop.example"}
	service.OrderPlaced(context.Background(), 160)
	service.OrderPlaced(context.Background(), 160)
	if len(store.rows) != 1 {
		t.Fatalf("expected one queued email, got %d", len(store.rows))
	}
	out := store.rows["order_placed:160"]
	for _, want := range []string{"ORD-000160", "Noor Gold Rim", "৳2,420", "https://shop.example/track", "pay ৳130 by bKash"} {
		if !strings.Contains(out.Text, want) {
			t.Fatalf("email lacks %q:\n%s", want, out.Text)
		}
	}
	for _, private := range []string{"01712345678", "Agrabad", "Rahim"} {
		if strings.Contains(out.Text, private) || strings.Contains(out.Subject, private) {
			t.Fatalf("order-placed email exposes %q", private)
		}
	}
	if out.To != "buyer@example.com" || out.InvoiceOrderID != 0 {
		t.Fatalf("unexpected email: %+v", out)
	}
}

// TestOrderNumbersEmail lists only numbers and dates, keys the queue by the email's HMAC and sends one per window.
func TestOrderNumbersEmail(t *testing.T) {
	store := newMemoryStore()
	service := Service{Store: store, SiteURL: "https://shop.example"}
	list := []orders.EmailedOrder{{OrderNumber: "ORD-000123", PlacedAt: time.Date(2026, 11, 3, 20, 0, 0, 0, time.UTC)}}
	key := strings.Repeat("ab", 32)
	service.OrderNumbers(context.Background(), "rahim@example.com", key, list)
	service.OrderNumbers(context.Background(), "rahim@example.com", key, list)
	if len(store.rows) != 1 {
		t.Fatalf("a repeat in the same window must not queue again, got %d rows", len(store.rows))
	}
	for k, out := range store.rows {
		if !strings.HasPrefix(k, "order_numbers:"+key[:24]+":") || strings.Contains(k, "rahim") {
			t.Fatalf("queue key %q", k)
		}
		// 20:00 UTC is 2:00 on 4 Nov in Dhaka.
		for _, want := range []string{"ORD-000123, placed 4 Nov 2026", "https://shop.example/track"} {
			if !strings.Contains(out.Text, want) {
				t.Fatalf("email lacks %q:\n%s", want, out.Text)
			}
		}
		if out.To != "rahim@example.com" {
			t.Fatalf("to = %q", out.To)
		}
	}
}

// TestCallbackEmail alerts the owner once per request with the phone, the frame link and the lens needs,
// and queues nothing when no owner email is set.
func TestCallbackEmail(t *testing.T) {
	store := newMemoryStore()
	service := Service{Store: store, SiteURL: "https://shop.example"}
	r := callbacks.Request{ID: 7, Name: "Rahim", Phone: "01712345678", ProductSlug: "eid-gold", Message: "-8.50 both eyes"}
	service.CallbackRequested(context.Background(), r)
	if len(store.rows) != 0 {
		t.Fatalf("queued without an owner email: %d", len(store.rows))
	}
	service.OwnerEmail = "owner@example.com"
	service.CallbackRequested(context.Background(), r)
	service.CallbackRequested(context.Background(), r)
	out, ok := store.rows["owner_callback:7"]
	if !ok || len(store.rows) != 1 || out.To != "owner@example.com" {
		t.Fatalf("queued = %+v", store.rows)
	}
	for _, want := range []string{"01712345678", "https://shop.example/products/eid-gold", "-8.50 both eyes", "https://shop.example/admin/callbacks"} {
		if !strings.Contains(out.Text, want) {
			t.Fatalf("email lacks %q:\n%s", want, out.Text)
		}
	}
}

// TestReturnEmail alerts the owner once per ticket with the order, the customer's phone and what they wrote.
func TestReturnEmail(t *testing.T) {
	store := newMemoryStore()
	service := Service{Store: store, SiteURL: "https://shop.example", OwnerEmail: "owner@example.com"}
	ticket := returns.Ticket{ID: 4, OrderNumber: "ORD-000200", CustomerName: "Rahim", CustomerPhone: "01712345678", Kind: returns.KindDefect, Message: "left hinge is loose", PhotoCount: 2}
	service.ReturnRequested(context.Background(), ticket)
	service.ReturnRequested(context.Background(), ticket)
	out, ok := store.rows["owner_return:4"]
	if !ok || len(store.rows) != 1 || out.To != "owner@example.com" || !strings.Contains(out.Subject, "defect or colour problem") {
		t.Fatalf("queued = %+v", store.rows)
	}
	for _, want := range []string{"ORD-000200", "01712345678", "left hinge is loose", "Photos: 2", "https://shop.example/admin/returns"} {
		if !strings.Contains(out.Text, want) {
			t.Fatalf("email lacks %q:\n%s", want, out.Text)
		}
	}
}

// TestInvoiceWaitsForConfirmedOrderAndEmail sends the invoice only after both the confirmation and the staff tick.
func TestInvoiceWaitsForConfirmedOrderAndEmail(t *testing.T) {
	order := sampleOrder()
	source := &fixedOrders{order}
	store := newMemoryStore()
	service := Service{Store: store, Orders: source, SiteURL: "https://shop.example"}

	source.order.Status = "confirmed"
	service.StatusChanged(context.Background(), 160, "confirmed")
	if len(store.rows) != 0 {
		t.Fatal("invoice sent before staff confirmed the email")
	}
	source.order.Status, source.order.EmailConfirmed = "pending", true
	service.EmailConfirmed(context.Background(), 160)
	if len(store.rows) != 0 {
		t.Fatal("invoice sent before the order was confirmed")
	}
	source.order.Status = "confirmed"
	service.StatusChanged(context.Background(), 160, "confirmed")
	service.EmailConfirmed(context.Background(), 160)
	out, ok := store.rows["invoice:160"]
	if !ok || len(store.rows) != 1 || out.InvoiceOrderID != 160 || !strings.Contains(out.Text, "To pay on delivery: ৳2,420") {
		t.Fatalf("expected one invoice email: %+v", store.rows)
	}
}

// TestStatusEmails queues shipped (with courier and cash due) and delivered, and nothing for other states.
func TestStatusEmails(t *testing.T) {
	order := sampleOrder()
	order.AmountPaid = 130
	order.Tracking = orders.Tracking{Courier: "Steadfast", TrackingCode: "SF-1", TrackingURL: "https://steadfast.example/t/SF-1"}
	store := newMemoryStore()
	service := Service{Store: store, Orders: &fixedOrders{order}, SiteURL: "https://shop.example"}
	for _, status := range []string{"preparing", "cancelled", "shipped", "delivered"} {
		service.StatusChanged(context.Background(), 160, status)
	}
	if len(store.rows) != 2 {
		t.Fatalf("expected shipped and delivered only, got %v", len(store.rows))
	}
	shipped := store.rows["shipped:160"].Text
	if !strings.Contains(shipped, "Steadfast") || !strings.Contains(shipped, "SF-1") || !strings.Contains(shipped, "৳2,290 ready") {
		t.Fatalf("shipped email lacks tracking or cash due:\n%s", shipped)
	}
	if _, ok := store.rows["delivered:160"]; !ok {
		t.Fatal("delivered email missing")
	}
}

// TestPaymentEmails alerts the owner only when configured and explains a rejection to the customer.
func TestPaymentEmails(t *testing.T) {
	store := newMemoryStore()
	store.payments[57] = PaymentNotice{OrderID: 160, Amount: 130, Status: "rejected", RejectReason: "TrxID not found", TrxID: "9ABC12XY34", SenderNumber: "01712345678"}
	service := Service{Store: store, Orders: &fixedOrders{sampleOrder()}, SiteURL: "https://shop.example"}
	service.PaymentSubmitted(context.Background(), 57)
	if len(store.rows) != 0 {
		t.Fatal("owner alert queued without an owner email")
	}
	service.OwnerEmail = "owner@example.com"
	service.PaymentSubmitted(context.Background(), 57)
	service.PaymentReviewed(context.Background(), 57)
	owner, customer := store.rows["owner_payment:57"], store.rows["payment_reviewed:57"]
	if owner.To != "owner@example.com" || !strings.Contains(owner.Text, "9ABC12XY34") || !strings.Contains(owner.Text, "/admin/payments") {
		t.Fatalf("unexpected owner alert: %+v", owner)
	}
	if customer.To != "buyer@example.com" || !strings.Contains(customer.Text, "TrxID not found") || !strings.Contains(customer.Subject, "could not accept") {
		t.Fatalf("unexpected rejection email: %+v", customer)
	}
}

// failingMail fails every send and counts calls.
type failingMail struct{ calls int }

// Send always fails.
func (m *failingMail) Send(context.Context, mail.Message) error {
	m.calls++
	return errors.New("provider down")
}

// okMail accepts every send.
type okMail struct{ sent []mail.Message }

// Send records the message.
func (m *okMail) Send(_ context.Context, msg mail.Message) error {
	m.sent = append(m.sent, msg)
	return nil
}

// TestWorkerRetriesThenFails backs off after a provider failure and stops at the attempt limit.
func TestWorkerRetriesThenFails(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	provider := &failingMail{}
	w := Worker{Store: store, Mail: provider}
	w.send(context.Background(), Queued{ID: 1, To: "buyer@example.com", Subject: "Hi", Text: "x", Attempts: 1}, now)
	w.send(context.Background(), Queued{ID: 1, To: "buyer@example.com", Subject: "Hi", Text: "x", Attempts: 2}, now)
	w.send(context.Background(), Queued{ID: 1, To: "buyer@example.com", Subject: "Hi", Text: "x", Attempts: maxAttempts}, now)
	if strings.Join(store.marks, ",") != "retry 10:01,retry 10:05,failed" {
		t.Fatalf("unexpected outcomes: %v", store.marks)
	}
	ok := &okMail{}
	w.Mail = ok
	w.send(context.Background(), Queued{ID: 2, To: "buyer@example.com", Subject: "Hi", Text: "x", Attempts: 1}, now)
	w.send(context.Background(), Queued{ID: 3, To: "e2e@example.test", Subject: "Hi", Text: "x", Attempts: 1}, now)
	if len(ok.sent) != 1 || strings.Join(store.marks[3:], ",") != "sent,skipped" {
		t.Fatalf("expected one send and a skipped test address: %v", store.marks)
	}
}

// TestSummaryOncePerDayAfterNine queues the summary at 9 PM Bangladesh time, once, and skips a quiet day.
func TestSummaryOncePerDayAfterNine(t *testing.T) {
	store := newMemoryStore()
	store.summary = DailySummary{NewOrderCount: 1, NewOrdersTotal: 2420, NewOrders: []SummaryOrder{{"ORD-000160", 2420, "pending"}}, PaymentsWaiting: 1}
	w := Worker{Store: store, OwnerEmail: "owner@example.com", SiteURL: "https://shop.example"}
	beforeNine := time.Date(2026, 11, 7, 14, 59, 0, 0, time.UTC) // 20:59 in Dhaka
	w.queueSummary(context.Background(), beforeNine)
	if len(store.rows) != 0 {
		t.Fatal("summary queued before 9 PM")
	}
	atNine := beforeNine.Add(time.Minute)
	w.queueSummary(context.Background(), atNine)
	store.summary.NewOrderCount = 99
	w.queueSummary(context.Background(), atNine.Add(time.Hour))
	out, ok := store.rows["owner_summary:2026-11-07"]
	if !ok || len(store.rows) != 1 || out.Skip || !strings.Contains(out.Text, "New orders today: 1 (৳2,420)") || !strings.Contains(out.Text, "bKash payments to check: 1") {
		t.Fatalf("expected one summary: %+v", store.rows)
	}
	store.summary = DailySummary{}
	w.queueSummary(context.Background(), atNine.Add(24*time.Hour))
	if quiet := store.rows["owner_summary:2026-11-08"]; !quiet.Skip {
		t.Fatal("a quiet day should be recorded without sending")
	}
}

// TestTaka formats amounts like the storefront.
func TestTaka(t *testing.T) {
	for amount, want := range map[int]string{0: "৳0", 130: "৳130", 2420: "৳2,420", 1234567: "৳1,234,567", -50: "-৳50"} {
		if got := taka(amount); got != want {
			t.Fatalf("taka(%d) = %q, want %q", amount, got, want)
		}
	}
}
