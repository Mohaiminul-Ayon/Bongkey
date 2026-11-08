package notifications

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"bongkey/backend/internal/invoice"
	"bongkey/backend/internal/mail"
)

// Queued is one claimed email ready to send.
type Queued struct {
	ID             int64
	To             string
	Subject        string
	Text           string
	InvoiceOrderID int
	// Attempts counts this attempt too.
	Attempts int
}

// DailySummary is what the owner's evening email reports.
type DailySummary struct {
	// NewOrders lists the day's first orders; NewOrderCount and NewOrdersTotal cover all of them.
	NewOrders       []SummaryOrder
	NewOrderCount   int
	NewOrdersTotal  int
	PaymentsWaiting int
	Prescriptions   int
	LensPrices      int
	NotShipped      int
	FailedEmails    int
}

// SummaryOrder is one order placed today.
type SummaryOrder struct {
	OrderNumber string
	Total       int
	Status      string
}

// quiet reports a day with nothing new and nothing waiting, which sends no summary.
func (d DailySummary) quiet() bool {
	return d.NewOrderCount == 0 && d.PaymentsWaiting == 0 && d.Prescriptions == 0 && d.LensPrices == 0 && d.NotShipped == 0 && d.FailedEmails == 0
}

// Store persists queued emails and reads what the emails report.
type Store interface {
	Enqueue(ctx context.Context, out Outgoing) error
	HasEvent(ctx context.Context, key string) (bool, error)
	Payment(ctx context.Context, paymentID int) (PaymentNotice, error)
	// ClaimDue marks up to limit due emails as sending, including ones left sending by a crash, and returns them.
	ClaimDue(ctx context.Context, limit int) ([]Queued, error)
	MarkSent(ctx context.Context, id int64) error
	MarkSkipped(ctx context.Context, id int64) error
	MarkRetry(ctx context.Context, id int64, next time.Time, reason string) error
	MarkFailed(ctx context.Context, id int64, reason string) error
	Summary(ctx context.Context, from, to time.Time) (DailySummary, error)
}

// retryDelays spread attempts over about a day, which also outlasts a provider's daily sending limit.
var retryDelays = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 6 * time.Hour, 6 * time.Hour}

// maxAttempts ends retrying with a final failed state that the owner's summary reports.
var maxAttempts = len(retryDelays) + 1

// dhaka is Bangladesh time; the country has no daylight saving, so a fixed zone needs no tz database.
var dhaka = time.FixedZone("Asia/Dhaka", 6*60*60)

// summaryHour is 9 PM Bangladesh time, when the owner's daily summary is queued.
const summaryHour = 21

// testDomain is reserved and never receives mail (RFC 2606); local test orders use it.
const testDomain = "@example.test"

// Worker sends due emails in small batches and queues the owner's daily summary.
type Worker struct {
	Store  Store
	Mail   mail.Sender
	Orders Orders
	// OwnerEmail receives the daily summary; empty turns it off.
	OwnerEmail string
	SiteURL    string
	// Interval is how often the queue is checked; Now is replaced in tests.
	Interval time.Duration
	Now      func() time.Time
}

// Run checks the queue until ctx is cancelled. One worker per server is enough for a small shop;
// several servers are safe because claiming skips rows another server has locked.
func (w Worker) Run(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		w.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tick queues the summary when due, then sends one batch of due emails.
func (w Worker) Tick(ctx context.Context) {
	now := time.Now()
	if w.Now != nil {
		now = w.Now()
	}
	w.queueSummary(ctx, now)
	queued, err := w.Store.ClaimDue(ctx, 20)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("notification queue unavailable")
		}
		return
	}
	for _, q := range queued {
		if ctx.Err() != nil {
			// Unsent claimed rows are picked up again after the stale-claim window.
			return
		}
		w.send(ctx, q, now)
	}
}

// send delivers one email and records the outcome; the record uses its own short context so a stop signal
// during sending still leaves an accurate state.
func (w Worker) send(ctx context.Context, q Queued, now time.Time) {
	record, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if strings.HasSuffix(strings.ToLower(q.To), testDomain) {
		_ = w.Store.MarkSkipped(record, q.ID)
		return
	}
	message := mail.Message{To: q.To, Subject: q.Subject, Text: q.Text}
	err := error(nil)
	if q.InvoiceOrderID > 0 {
		message.Attachment, err = w.invoice(ctx, q.InvoiceOrderID)
	}
	if err == nil {
		sendCtx, cancelSend := context.WithTimeout(ctx, 30*time.Second)
		err = w.Mail.Send(sendCtx, message)
		cancelSend()
	}
	if err == nil {
		if markErr := w.Store.MarkSent(record, q.ID); markErr != nil {
			log.Printf("notification %d sent but not recorded", q.ID)
		}
		return
	}
	reason := "send failed"
	if q.Attempts >= maxAttempts {
		_ = w.Store.MarkFailed(record, q.ID, reason)
		log.Printf("notification %d failed after %d attempts", q.ID, q.Attempts)
		return
	}
	delay := retryDelays[min(q.Attempts, len(retryDelays))-1]
	_ = w.Store.MarkRetry(record, q.ID, now.Add(delay), reason)
}

// invoice renders the order's current invoice PDF for the attachment.
func (w Worker) invoice(ctx context.Context, orderID int) (*mail.Attachment, error) {
	order, err := w.Orders.Admin(ctx, strconv.Itoa(orderID))
	if err != nil {
		return nil, err
	}
	pdf, err := invoice.Render(order)
	if err != nil {
		return nil, err
	}
	return &mail.Attachment{Name: order.InvoiceNumber + ".pdf", ContentType: "application/pdf", Content: pdf}, nil
}

// queueSummary queues today's owner summary once, at or after 9 PM. A server that starts later the same
// evening still sends it; a restart never sends it twice, because the day's event key is unique.
func (w Worker) queueSummary(ctx context.Context, now time.Time) {
	if w.OwnerEmail == "" {
		return
	}
	local := now.In(dhaka)
	if local.Hour() < summaryHour {
		return
	}
	key := "owner_summary:" + local.Format("2006-01-02")
	if done, err := w.Store.HasEvent(ctx, key); err != nil || done {
		return
	}
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, dhaka)
	summary, err := w.Store.Summary(ctx, dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		log.Printf("owner summary not built")
		return
	}
	out := Outgoing{Key: key, To: w.OwnerEmail, Subject: "Bongkey summary for " + local.Format("2 Jan 2006"), Text: summaryText(summary, w.SiteURL), Skip: summary.quiet()}
	if err := w.Store.Enqueue(ctx, out); err != nil {
		log.Printf("notification not queued: event=%s", key)
	}
}

// summaryText lists today's orders and everything waiting for the owner.
func summaryText(d DailySummary, siteURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "New orders today: %d (%s)\n", d.NewOrderCount, taka(d.NewOrdersTotal))
	for _, order := range d.NewOrders {
		fmt.Fprintf(&b, "- %s  %s  %s\n", order.OrderNumber, taka(order.Total), order.Status)
	}
	if more := d.NewOrderCount - len(d.NewOrders); more > 0 {
		fmt.Fprintf(&b, "- and %d more\n", more)
	}
	fmt.Fprintf(&b, "\nWaiting for you:\n- bKash payments to check: %d\n- Prescriptions to check: %d\n- Lens prices to confirm: %d\n- Confirmed orders not shipped: %d\n", d.PaymentsWaiting, d.Prescriptions, d.LensPrices, d.NotShipped)
	if d.FailedEmails > 0 {
		fmt.Fprintf(&b, "- Emails that could not be sent (last 24 hours): %d\n", d.FailedEmails)
	}
	fmt.Fprintf(&b, "\nOpen the admin: %s/admin/orders\n", siteURL)
	return b.String()
}
