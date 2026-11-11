package returns

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// storeStub keeps tickets in memory and records what reached it.
type storeStub struct {
	tickets  map[int64]Ticket
	created  int
	failSave bool
	updates  []string
	refunds  []Refund
}

func (s *storeStub) Create(_ context.Context, orderID int, kind, message string, photos []string) (Ticket, error) {
	if s.failSave {
		return Ticket{}, errors.New("database down")
	}
	s.created++
	t := Ticket{ID: int64(s.created), OrderID: orderID, Kind: kind, Message: message, Photos: photos, PhotoCount: len(photos), Status: StatusOpen}
	if s.tickets == nil {
		s.tickets = map[int64]Ticket{}
	}
	s.tickets[t.ID] = t
	return t, nil
}
func (s *storeStub) List(context.Context, string, int) ([]Ticket, error) {
	var list []Ticket
	for _, t := range s.tickets {
		list = append(list, t)
	}
	return list, nil
}
func (s *storeStub) Get(_ context.Context, id int64) (Ticket, error) {
	t, ok := s.tickets[id]
	if !ok {
		return Ticket{}, ErrNotFound
	}
	return t, nil
}
func (s *storeStub) Update(_ context.Context, _ int64, status, outcome, _ string) error {
	s.updates = append(s.updates, status+"/"+outcome)
	return nil
}
func (s *storeStub) RecordRefund(_ context.Context, _ int64, r Refund, _ string) error {
	s.refunds = append(s.refunds, r)
	return nil
}

// photoStub saves photos under counted names, can fail on a given call and records removals.
type photoStub struct {
	saved   int
	failOn  int
	removed []string
}

func (p *photoStub) Save([]byte) (string, error) {
	p.saved++
	if p.saved == p.failOn {
		return "", errors.New("bad photo")
	}
	return "photo" + string(rune('0'+p.saved)), nil
}
func (p *photoStub) Remove(name string)               { p.removed = append(p.removed, name) }
func (p *photoStub) Path(name string) (string, error) { return "/private/" + name, nil }

// notifierStub records owner alerts.
type notifierStub struct{ sent []Ticket }

func (n *notifierStub) ReturnRequested(_ context.Context, t Ticket) { n.sent = append(n.sent, t) }

var delivered = time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)

// at returns a clock fixed at delivery plus the given time.
func at(after time.Duration) func() time.Time {
	return func() time.Time { return delivered.Add(after) }
}

// TestEligibleWindows applies 3 days for an exchange and 15 days for a problem, counted from delivery.
func TestEligibleWindows(t *testing.T) {
	order := Order{ID: 1, Status: "delivered", DeliveredAt: &delivered}
	for name, tc := range map[string]struct {
		order Order
		kind  string
		after time.Duration
		want  error
	}{
		"exchange on day 2":        {order, KindExchange, 2 * day, nil},
		"exchange at exactly 3d":   {order, KindExchange, 3 * day, nil},
		"exchange on day 4":        {order, KindExchange, 3*day + time.Minute, ErrWindowClosed},
		"defect on day 14":         {order, KindDefect, 14 * day, nil},
		"wrong item on day 16":     {order, KindWrongItem, 16 * day, ErrWindowClosed},
		"not delivered yet":        {Order{ID: 1, Status: "shipped"}, KindDefect, 0, ErrNotDelivered},
		"cancelled after delivery": {Order{ID: 1, Status: "cancelled", DeliveredAt: &delivered}, KindDefect, day, ErrNotDelivered},
		"unknown kind":             {order, "refund", day, ErrInvalid},
	} {
		if err := Eligible(tc.order, tc.kind, delivered.Add(tc.after)); !errors.Is(err, tc.want) {
			t.Fatalf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

// TestCreateSavesPhotosAndAlerts trims the message, stores each photo and alerts the owner once.
func TestCreateSavesPhotosAndAlerts(t *testing.T) {
	store, photos, alerts := &storeStub{}, &photoStub{}, &notifierStub{}
	service := Service{Store: store, Photos: photos, Notifier: alerts, Now: at(day)}
	order := Order{ID: 7, Status: "delivered", DeliveredAt: &delivered}
	ticket, err := service.Create(context.Background(), order, Submission{Kind: KindDefect, Message: "  left hinge is loose  ", Photos: [][]byte{{1}, {2}}})
	if err != nil || ticket.Message != "left hinge is loose" || ticket.PhotoCount != 2 || ticket.OrderID != 7 {
		t.Fatalf("ticket = %+v %v", ticket, err)
	}
	if len(alerts.sent) != 1 {
		t.Fatalf("owner alerts = %d", len(alerts.sent))
	}
}

// TestCreateRefusesAndCleansUp refuses bad requests before any file is kept, and removes saved photos when a
// later photo or the database fails.
func TestCreateRefusesAndCleansUp(t *testing.T) {
	order := Order{ID: 7, Status: "delivered", DeliveredAt: &delivered}
	for name, sub := range map[string]Submission{
		"short message":  {Kind: KindDefect, Message: "bad"},
		"long message":   {Kind: KindDefect, Message: strings.Repeat("x", 1001)},
		"too many files": {Kind: KindDefect, Message: "hinge broke", Photos: [][]byte{{1}, {2}, {3}, {4}}},
	} {
		photos := &photoStub{}
		service := Service{Store: &storeStub{}, Photos: photos, Now: at(day)}
		if _, err := service.Create(context.Background(), order, sub); !errors.Is(err, ErrInvalid) || photos.saved != 0 {
			t.Fatalf("%s: err = %v, saved %d", name, err, photos.saved)
		}
	}
	photos := &photoStub{failOn: 2}
	service := Service{Store: &storeStub{}, Photos: photos, Now: at(day)}
	if _, err := service.Create(context.Background(), order, Submission{Kind: KindDefect, Message: "hinge broke", Photos: [][]byte{{1}, {2}}}); err == nil || len(photos.removed) != 1 {
		t.Fatalf("bad second photo: err = %v, removed %v", err, photos.removed)
	}
	photos = &photoStub{}
	service = Service{Store: &storeStub{failSave: true}, Photos: photos, Now: at(day)}
	if _, err := service.Create(context.Background(), order, Submission{Kind: KindDefect, Message: "hinge broke", Photos: [][]byte{{1}}}); err == nil || len(photos.removed) != 1 {
		t.Fatalf("database failure: err = %v, removed %v", err, photos.removed)
	}
	service = Service{Store: &storeStub{}, Photos: &photoStub{}, Now: at(4 * day)}
	if _, err := service.Create(context.Background(), order, Submission{Kind: KindExchange, Message: "want the black one"}); !errors.Is(err, ErrWindowClosed) {
		t.Fatalf("late exchange: %v", err)
	}
}

// TestUpdateRules needs an outcome only when closing, keeps "refunded" for RecordRefund and asks why on a decline.
func TestUpdateRules(t *testing.T) {
	store := &storeStub{}
	service := Service{Store: store}
	ctx := context.Background()
	for name, tc := range map[string]struct {
		status, outcome, note string
		ok                    bool
	}{
		"in progress":              {StatusInProgress, "", "", true},
		"close as exchanged":       {StatusClosed, OutcomeExchanged, "", true},
		"decline with a note":      {StatusClosed, OutcomeDeclined, "scratches from use", true},
		"decline without a note":   {StatusClosed, OutcomeDeclined, " ", false},
		"close without an outcome": {StatusClosed, "", "", false},
		"close as refunded":        {StatusClosed, OutcomeRefunded, "", false},
		"outcome while open":       {StatusOpen, OutcomeExchanged, "", false},
		"unknown status":           {"lost", "", "", false},
	} {
		err := service.Update(ctx, 1, tc.status, tc.outcome, tc.note)
		if (err == nil) != tc.ok || (!tc.ok && !errors.Is(err, ErrInvalid)) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	if len(store.updates) != 3 {
		t.Fatalf("store updates = %v", store.updates)
	}
}

// TestRecordRefundRules refunds only open problem tickets, within the order total, with a TrxID for bKash and
// never with a future date.
func TestRecordRefundRules(t *testing.T) {
	store := &storeStub{tickets: map[int64]Ticket{
		1: {ID: 1, Kind: KindDefect, Status: StatusInProgress, OrderTotal: 3000, CreatedAt: delivered},
		2: {ID: 2, Kind: KindExchange, Status: StatusOpen, OrderTotal: 3000, CreatedAt: delivered},
		3: {ID: 3, Kind: KindWrongItem, Status: StatusClosed, OrderTotal: 3000, CreatedAt: delivered},
	}}
	service := Service{Store: store, Now: at(5 * day)}
	ctx := context.Background()
	good := Refund{Amount: 2810, Method: "bkash", Reference: "9ABC12XY34"}
	for name, tc := range map[string]struct {
		id   int64
		r    Refund
		want error
	}{
		"exchange ticket":   {2, good, ErrNotRefundable},
		"closed ticket":     {3, good, ErrNotRefundable},
		"above order total": {1, Refund{Amount: 3001, Method: "cash"}, ErrInvalid},
		"bKash without ID":  {1, Refund{Amount: 100, Method: "bkash"}, ErrInvalid},
		"zero amount":       {1, Refund{Method: "cash"}, ErrInvalid},
		"future date":       {1, Refund{Amount: 100, Method: "cash", PaidAt: delivered.Add(6 * day)}, ErrInvalid},
		"missing ticket":    {9, good, ErrNotFound},
	} {
		if err := service.RecordRefund(ctx, tc.id, tc.r, ""); !errors.Is(err, tc.want) {
			t.Fatalf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if err := service.RecordRefund(ctx, 1, good, ""); err != nil || len(store.refunds) != 1 || !store.refunds[0].PaidAt.Equal(delivered.Add(5*day)) {
		t.Fatalf("good refund: %v %+v", err, store.refunds)
	}
}

// TestRefundDeadlineFromRequest counts 15 days from the request (not approval) and flags an open ticket after it.
func TestRefundDeadlineFromRequest(t *testing.T) {
	requested := delivered.Add(2 * day)
	store := &storeStub{tickets: map[int64]Ticket{
		1: {ID: 1, Kind: KindDefect, Status: StatusInProgress, CreatedAt: requested},
		2: {ID: 2, Kind: KindExchange, Status: StatusOpen, CreatedAt: requested},
		3: {ID: 3, Kind: KindDefect, Status: StatusClosed, CreatedAt: requested},
	}}
	for after, overdue := range map[time.Duration]bool{14 * day: false, 15*day + time.Minute: true} {
		service := Service{Store: store, Now: func() time.Time { return requested.Add(after) }}
		problem, _ := service.Get(context.Background(), 1)
		if problem.RefundDue == nil || !problem.RefundDue.Equal(requested.Add(15*day)) || problem.Overdue != overdue {
			t.Fatalf("after %v: due %v overdue %v", after, problem.RefundDue, problem.Overdue)
		}
		exchange, _ := service.Get(context.Background(), 2)
		closed, _ := service.Get(context.Background(), 3)
		if exchange.RefundDue != nil || exchange.Overdue || closed.Overdue {
			t.Fatalf("after %v: exchange %+v closed %+v", after, exchange, closed)
		}
	}
}
