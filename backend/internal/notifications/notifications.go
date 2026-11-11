// Package notifications queues customer and owner emails as database rows and sends them in the background,
// so a slow or failing email provider never blocks or rolls back an order.
package notifications

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"bongkey/backend/internal/callbacks"
	"bongkey/backend/internal/orders"
)

// Outgoing is one email to queue; Key names the business event so the same event is queued only once.
type Outgoing struct {
	Key     string
	To      string
	Subject string
	Text    string
	// InvoiceOrderID attaches that order's invoice PDF, rendered when the email is sent.
	InvoiceOrderID int
	// Skip records the event as handled without sending, such as a quiet day's summary.
	Skip bool
}

// PaymentNotice is what the payment emails need about one submitted bKash payment.
type PaymentNotice struct {
	OrderID      int
	Amount       int
	Status       string
	RejectReason string
	TrxID        string
	SenderNumber string
}

// Orders reads one order with its customer email and items; orders.AccessService implements it.
type Orders interface {
	Admin(ctx context.Context, id string) (orders.AdminDetail, error)
}

// Service turns order and payment events into queued emails. Its methods never return errors to the
// business change that triggered them; a failure to queue is logged without customer data.
type Service struct {
	Store  Store
	Orders Orders
	// OwnerEmail receives payment alerts; empty turns owner emails off.
	OwnerEmail string
	// SiteURL is the storefront address used in links, without a trailing slash.
	SiteURL string
}

// queueTimeout bounds one enqueue after the business change committed.
const queueTimeout = 5 * time.Second

// detached keeps the enqueue alive when the customer's request is cancelled right after the order committed:
// the order exists, so its email must still be queued.
func detached(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), queueTimeout)
}

// OrderPlaced queues the order-received email. It shows the order number, products and totals but no
// address or phone, because the email address was not verified at checkout and may belong to someone else.
func (s Service) OrderPlaced(ctx context.Context, orderID int) {
	ctx, cancel := detached(ctx)
	defer cancel()
	order, ok := s.order(ctx, orderID, "order_placed")
	if !ok || order.CustomerEmail == "" {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Hello,\n\nThank you for your order at Bongkey.\n\nOrder number: %s\n", order.OrderNumber)
	for _, item := range order.Items {
		fmt.Fprintf(&b, "- %s (%s) x%d: %s\n", item.ProductName, item.LensOption, item.Quantity, taka(item.TotalPrice))
	}
	fmt.Fprintf(&b, "Subtotal: %s\nDelivery: %s\nTotal: %s\n\nWhat happens next: %s\n\n", taka(order.Subtotal), taka(order.DeliveryFee), taka(order.Total), nextStep(order))
	fmt.Fprintf(&b, "Track your order any time at %s/track with your order number and phone number.\n\nIf you did not place this order, you can ignore this email.\n\nBongkey", s.SiteURL)
	s.enqueue(ctx, Outgoing{Key: event("order_placed", orderID), To: order.CustomerEmail, Subject: "Bongkey order " + order.OrderNumber + " received", Text: b.String()})
}

// nextStep tells the customer what we do next, matching the payment rule saved on the order.
func nextStep(order orders.AdminDetail) string {
	due := order.AdvanceRequired - order.AmountPaid
	switch {
	case order.LensPricePending:
		return "We check your lens details and call you to confirm the final lens price. Then you pay by bKash; we start making your lenses after that."
	case order.LensOrder && due > 0:
		return fmt.Sprintf("We check your lens details and call you. Then pay %s by bKash (open the Track page and choose Pay by bKash); we start making your lenses after that.", taka(due))
	case due > 0:
		return fmt.Sprintf("Please pay %s by bKash (open the Track page and choose Pay by bKash). We confirm the order after that and you pay the rest on delivery.", taka(due))
	default:
		return "We will call you on your phone to confirm the order."
	}
}

// StatusChanged queues the email that belongs to a new order status, if any.
func (s Service) StatusChanged(ctx context.Context, orderID int, status string) {
	ctx, cancel := detached(ctx)
	defer cancel()
	switch status {
	case "confirmed":
		s.invoiceIfReady(ctx, orderID)
	case "shipped":
		order, ok := s.order(ctx, orderID, "shipped")
		if !ok || order.CustomerEmail == "" {
			return
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Hello,\n\nYour Bongkey order %s is on the way.\n", order.OrderNumber)
		if t := order.Tracking; t.Courier != "" || t.TrackingCode != "" {
			fmt.Fprintf(&b, "Courier: %s\nTracking code: %s\n", t.Courier, t.TrackingCode)
			if t.TrackingURL != "" {
				fmt.Fprintf(&b, "Track with the courier: %s\n", t.TrackingURL)
			}
		}
		if rest := order.Total - order.AmountPaid; rest > 0 {
			fmt.Fprintf(&b, "\nPlease keep %s ready to pay on delivery.\n", taka(rest))
		}
		fmt.Fprintf(&b, "\nYou can also follow it at %s/track with your order number and phone number.\n\nBongkey", s.SiteURL)
		s.enqueue(ctx, Outgoing{Key: event("shipped", orderID), To: order.CustomerEmail, Subject: "Bongkey order " + order.OrderNumber + " is on the way", Text: b.String()})
	case "delivered":
		order, ok := s.order(ctx, orderID, "delivered")
		if !ok || order.CustomerEmail == "" {
			return
		}
		text := fmt.Sprintf("Hello,\n\nYour Bongkey order %s was delivered. Thank you for shopping with us!\n\nIf anything is wrong with your glasses, reply to this email or message us on WhatsApp with your order number.\n\nBongkey", order.OrderNumber)
		s.enqueue(ctx, Outgoing{Key: event("delivered", orderID), To: order.CustomerEmail, Subject: "Bongkey order " + order.OrderNumber + " delivered", Text: text})
	}
}

// EmailConfirmed lets the invoice go out once staff ticked that the email was read back on the call.
func (s Service) EmailConfirmed(ctx context.Context, orderID int) {
	ctx, cancel := detached(ctx)
	defer cancel()
	s.invoiceIfReady(ctx, orderID)
}

// invoiceIfReady queues the invoice email only for a confirmed order whose email staff confirmed;
// whichever of the two happens second sends it, and the event key keeps it to one email.
func (s Service) invoiceIfReady(ctx context.Context, orderID int) {
	order, ok := s.order(ctx, orderID, "invoice")
	if !ok || order.CustomerEmail == "" || !order.EmailConfirmed {
		return
	}
	switch order.Status {
	case "confirmed", "preparing", "shipped", "delivered":
	default:
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Hello,\n\nYour Bongkey order %s is confirmed. Your invoice %s is attached.\n\nTotal: %s\nPaid: %s\n", order.OrderNumber, order.InvoiceNumber, taka(order.Total), taka(order.AmountPaid))
	if rest := order.Total - order.AmountPaid; rest > 0 {
		fmt.Fprintf(&b, "To pay on delivery: %s\n", taka(rest))
	}
	fmt.Fprintf(&b, "\nFollow your order at %s/track with your order number and phone number.\n\nBongkey", s.SiteURL)
	s.enqueue(ctx, Outgoing{Key: event("invoice", orderID), To: order.CustomerEmail, Subject: "Bongkey invoice " + order.InvoiceNumber + " for order " + order.OrderNumber, Text: b.String(), InvoiceOrderID: orderID})
}

// PaymentSubmitted alerts the owner at once, because a waiting bKash payment holds the order.
func (s Service) PaymentSubmitted(ctx context.Context, paymentID int) {
	if s.OwnerEmail == "" {
		return
	}
	ctx, cancel := detached(ctx)
	defer cancel()
	payment, err := s.Store.Payment(ctx, paymentID)
	if err != nil {
		log.Printf("notification not queued: event=%s", event("owner_payment", paymentID))
		return
	}
	order, ok := s.order(ctx, payment.OrderID, "owner_payment")
	if !ok {
		return
	}
	text := fmt.Sprintf("A bKash payment is waiting for your check.\n\nOrder: %s (total %s)\nAmount: %s\nTrxID: %s\nSender: %s\n\nApprove or reject it at %s/admin/payments\n", order.OrderNumber, taka(order.Total), taka(payment.Amount), payment.TrxID, payment.SenderNumber, s.SiteURL)
	s.enqueue(ctx, Outgoing{Key: event("owner_payment", paymentID), To: s.OwnerEmail, Subject: "bKash payment waiting: " + order.OrderNumber + " " + taka(payment.Amount), Text: text})
}

// PaymentReviewed tells the customer whether their bKash payment was accepted.
func (s Service) PaymentReviewed(ctx context.Context, paymentID int) {
	ctx, cancel := detached(ctx)
	defer cancel()
	payment, err := s.Store.Payment(ctx, paymentID)
	if err != nil || (payment.Status != "approved" && payment.Status != "rejected") {
		log.Printf("notification not queued: event=%s", event("payment_reviewed", paymentID))
		return
	}
	order, ok := s.order(ctx, payment.OrderID, "payment_reviewed")
	if !ok || order.CustomerEmail == "" {
		return
	}
	subject := "Bongkey received your bKash payment for " + order.OrderNumber
	text := fmt.Sprintf("Hello,\n\nWe received your bKash payment of %s for order %s (TrxID %s). Thank you!\n\nPaid so far: %s of %s.\n\nBongkey", taka(payment.Amount), order.OrderNumber, payment.TrxID, taka(order.AmountPaid), taka(order.Total))
	if payment.Status == "rejected" {
		subject = "Bongkey could not accept your bKash payment for " + order.OrderNumber
		text = fmt.Sprintf("Hello,\n\nWe could not accept your bKash payment of %s for order %s (TrxID %s).\nReason: %s\n\nPlease check the transaction and send it again from the Track page at %s/track, or message us on WhatsApp with your order number.\n\nBongkey", taka(payment.Amount), order.OrderNumber, payment.TrxID, payment.RejectReason, s.SiteURL)
	}
	s.enqueue(ctx, Outgoing{Key: event("payment_reviewed", paymentID), To: order.CustomerEmail, Subject: subject, Text: text})
}

// CallbackRequested alerts the owner at once, because the shopper is waiting for a call about a custom lens.
// The phone and message go only to the owner's own inbox, never to a log.
func (s Service) CallbackRequested(ctx context.Context, r callbacks.Request) {
	if s.OwnerEmail == "" {
		return
	}
	ctx, cancel := detached(ctx)
	defer cancel()
	frame := "not given"
	if r.ProductSlug != "" {
		frame = s.SiteURL + "/products/" + r.ProductSlug
	}
	text := fmt.Sprintf("A customer asked to be called about a custom lens.\n\nName: %s\nPhone: %s\nFrame: %s\n\nTheir lens needs:\n%s\n\nMark it done after the call at %s/admin/callbacks\n", r.Name, r.Phone, frame, r.Message, s.SiteURL)
	s.enqueue(ctx, Outgoing{Key: "owner_callback:" + strconv.FormatInt(r.ID, 10), To: s.OwnerEmail, Subject: "Call back " + r.Name + " " + r.Phone + " about a custom lens", Text: text})
}

// orderNumbersWindow lets one email receive at most one reminder per window (three an hour), so repeated
// requests cannot flood an inbox.
const orderNumbersWindow = 20 * time.Minute

// OrderNumbers queues the "your order numbers" reminder for a guest who lost them. It lists only numbers and
// dates because the checkout email was never verified and may belong to someone else. emailKey is an HMAC of
// the address, so the queue key and logs never contain the email itself.
func (s Service) OrderNumbers(ctx context.Context, to, emailKey string, list []orders.EmailedOrder) {
	ctx, cancel := detached(ctx)
	defer cancel()
	var b strings.Builder
	b.WriteString("Hello,\n\nYou asked for the order numbers placed with this email at Bongkey:\n\n")
	for _, order := range list {
		fmt.Fprintf(&b, "- %s, placed %s\n", order.OrderNumber, order.PlacedAt.In(dhaka).Format("2 Jan 2006"))
	}
	fmt.Fprintf(&b, "\nTrack an order at %s/track with its order number and the phone number you ordered with.\n\nIf you did not ask for this, you can ignore this email.\n\nBongkey", s.SiteURL)
	window := time.Now().Truncate(orderNumbersWindow).Unix()
	key := fmt.Sprintf("order_numbers:%s:%d", emailKey[:24], window)
	s.enqueue(ctx, Outgoing{Key: key, To: to, Subject: "Your Bongkey order numbers", Text: b.String()})
}

// order loads one order for an email; a failure is logged by event and ID only.
func (s Service) order(ctx context.Context, orderID int, kind string) (orders.AdminDetail, bool) {
	order, err := s.Orders.Admin(ctx, strconv.Itoa(orderID))
	if err != nil {
		log.Printf("notification not queued: event=%s", event(kind, orderID))
		return orders.AdminDetail{}, false
	}
	return order, true
}

// enqueue saves one email; the unique event key turns a repeated event into a no-op.
func (s Service) enqueue(ctx context.Context, out Outgoing) {
	if err := s.Store.Enqueue(ctx, out); err != nil {
		log.Printf("notification not queued: event=%s", out.Key)
	}
}

// event builds the unique key for one business event, such as order_placed:160.
func event(kind string, id int) string {
	return kind + ":" + strconv.Itoa(id)
}

// taka formats whole taka with thousands separators, as the storefront does: ৳2,420.
func taka(amount int) string {
	sign := ""
	if amount < 0 {
		sign, amount = "-", -amount
	}
	digits := strconv.Itoa(amount)
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	return sign + "৳" + digits
}
