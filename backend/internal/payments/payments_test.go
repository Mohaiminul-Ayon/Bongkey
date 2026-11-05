package payments

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// storeStub serves fixed settings and order state and records what reached it.
type storeStub struct {
	Store
	method    Method
	state     OrderState
	list      []Payment
	submitted []Submission
	saved     []Method
}

func (s *storeStub) Method(context.Context, string) (Method, error) { return s.method, nil }
func (s *storeStub) OrderState(context.Context, int) (OrderState, error) {
	return s.state, nil
}
func (s *storeStub) ForOrder(context.Context, int) ([]Payment, error) { return s.list, nil }
func (s *storeStub) Submit(_ context.Context, _ int, sub Submission) (Payment, error) {
	s.submitted = append(s.submitted, sub)
	return Payment{TrxID: sub.TrxID, SenderNumber: sub.SenderNumber}, nil
}
func (s *storeStub) SaveMethod(_ context.Context, m Method) error {
	s.saved = append(s.saved, m)
	return nil
}

var dueState = OrderState{PaymentStatus: "advance_due", OrderStatus: "pending", Total: 5000, Required: 5000}

// TestSubmitNormalizesAndValidates accepts what customers really type and refuses malformed values before the store.
func TestSubmitNormalizesAndValidates(t *testing.T) {
	store := &storeStub{method: Method{Code: MethodBkash, Enabled: true, AccountNumber: "01711111111"}}
	service := Service{Store: store}
	got, err := service.Submit(context.Background(), 1, Submission{Method: "bkash", TrxID: " 9abc12xy34 ", SenderNumber: "+880 1712-345678", Amount: 1500})
	if err != nil || got.TrxID != "9ABC12XY34" || got.SenderNumber != "01712345678" {
		t.Fatalf("normalized submission = %+v %v", got, err)
	}
	for name, sub := range map[string]Submission{
		"short trx":       {Method: "bkash", TrxID: "ABC12", SenderNumber: "01712345678", Amount: 100},
		"trx with symbol": {Method: "bkash", TrxID: "9ABC-12XY34", SenderNumber: "01712345678", Amount: 100},
		"landline sender": {Method: "bkash", TrxID: "9ABC12XY34", SenderNumber: "0212345678", Amount: 100},
		"zero amount":     {Method: "bkash", TrxID: "9ABC12XY34", SenderNumber: "01712345678", Amount: 0},
		"negative amount": {Method: "bkash", TrxID: "9ABC12XY34", SenderNumber: "01712345678", Amount: -5},
		"other method":    {Method: "nagad", TrxID: "9ABC12XY34", SenderNumber: "01712345678", Amount: 100},
	} {
		if _, err := service.Submit(context.Background(), 1, sub); !errors.Is(err, ErrInvalidPayment) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	if len(store.submitted) != 1 {
		t.Fatalf("invalid submissions reached the store: %d", len(store.submitted))
	}
	store.method.Enabled = false
	if _, err := service.Submit(context.Background(), 1, Submission{Method: "bkash", TrxID: "9ABC12XY34", SenderNumber: "01712345678", Amount: 100}); !errors.Is(err, ErrMethodOff) {
		t.Fatalf("submission while bKash is off: %v", err)
	}
}

// TestPayableRules covers when a customer may send money and when the owner may approve it.
func TestPayableRules(t *testing.T) {
	with := func(change func(*OrderState)) OrderState {
		state := dueState
		change(&state)
		return state
	}
	for name, tc := range map[string]struct {
		state             OrderState
		amount            int
		payable, approval error
	}{
		"advance due":                     {dueState, 1000, nil, nil},
		"advance received, balance left":  {with(func(s *OrderState) { s.PaymentStatus, s.Required, s.Paid = "advance_received", 2000, 2000 }), 1000, nil, nil},
		"cash on delivery, pay early":     {with(func(s *OrderState) { s.PaymentStatus, s.Required = "unpaid", 0 }), 1000, nil, nil},
		"shipped, still payable":          {with(func(s *OrderState) { s.OrderStatus = "shipped" }), 1000, nil, nil},
		"delivered: approve a late check": {with(func(s *OrderState) { s.OrderStatus = "delivered" }), 1000, ErrNotPayable, nil},
		"paid in full":                    {with(func(s *OrderState) { s.Paid = 5000 }), 1, ErrNotPayable, ErrOverpaid},
		"approval above the total":        {with(func(s *OrderState) { s.Paid = 4500 }), 1000, nil, ErrOverpaid},
		"lens price open":                 {with(func(s *OrderState) { s.PriceOpen = true }), 1000, ErrPriceNotFinal, ErrPriceNotFinal},
		"cancelled order":                 {with(func(s *OrderState) { s.OrderStatus = "cancelled" }), 1000, ErrNotPayable, ErrNotPayable},
	} {
		if err := checkPayable(tc.state); !errors.Is(err, tc.payable) && err != tc.payable {
			t.Fatalf("%s: payable = %v, want %v", name, err, tc.payable)
		}
		if err := checkApprovable(tc.state, tc.amount); !errors.Is(err, tc.approval) && err != tc.approval {
			t.Fatalf("%s: approvable = %v, want %v", name, err, tc.approval)
		}
	}
}

// TestGuestViewShowsNumberOnlyWhenPayable keeps the receiving number off orders that must not be paid yet,
// and never shows the customer which other orders reuse a transaction ID.
func TestGuestViewShowsNumberOnlyWhenPayable(t *testing.T) {
	store := &storeStub{method: Method{Code: MethodBkash, Enabled: true, AccountNumber: "01711111111"}, state: dueState,
		list: []Payment{{ID: 1, UsedOnOrders: []string{"ORD-000009"}}}}
	view, err := Service{Store: store}.ForGuest(context.Background(), 1)
	if err != nil || view.Payable != "yes" || view.Method == nil || view.Method.AccountNumber != "01711111111" || view.Payments[0].UsedOnOrders != nil {
		t.Fatalf("payable view = %+v %v", view, err)
	}
	for want, state := range map[string]OrderState{"price_pending": {PaymentStatus: "advance_due", OrderStatus: "pending", Total: 5000, PriceOpen: true}, "not_due": {PaymentStatus: "advance_received", OrderStatus: "pending", Total: 5000, Paid: 5000}} {
		store.state = state
		if view, err := (Service{Store: store}).ForGuest(context.Background(), 1); err != nil || view.Payable != want || view.Method != nil {
			t.Fatalf("%s view = %+v %v", want, view, err)
		}
	}
	store.state, store.method.Enabled = dueState, false
	if view, _ := (Service{Store: store}).ForGuest(context.Background(), 1); view.Payable != "off" || view.Method != nil {
		t.Fatalf("view with bKash off = %+v", view)
	}
}

// TestSettles keeps an order due until approved payments cover the advance saved on it.
func TestSettles(t *testing.T) {
	for name, tc := range map[string]struct {
		state  OrderState
		amount int
		want   bool
	}{
		"part of a full payment":      {OrderState{Total: 2370, Required: 2370}, 500, false},
		"second part fills":           {OrderState{Total: 2370, Required: 2370, Paid: 500}, 1870, true},
		"delivery fee paid":           {OrderState{Total: 2370, Required: 80}, 80, true},
		"half advance, short by one":  {OrderState{Total: 6500, Required: 3250}, 3249, false},
		"more than the advance is ok": {OrderState{Total: 6500, Required: 3250}, 4000, true},
	} {
		if got := settles(tc.state, tc.amount); got != tc.want {
			t.Fatalf("%s: settles = %v", name, got)
		}
	}
}

// TestValidPolicy accepts only rules the database allows and refuses cash on delivery for lenses.
func TestValidPolicy(t *testing.T) {
	for p, want := range map[Policy]bool{
		{Kind: "lens", Mode: "full"}:                     true,
		{Kind: "lens", Mode: "percent", Value: 50}:       true,
		{Kind: "lens", Mode: "fixed", Value: 1000}:       true,
		{Kind: "lens", Mode: "none"}:                     false,
		{Kind: "lens", Mode: "delivery_fee"}:             false,
		{Kind: "lens", Mode: "percent", Value: 0}:        false,
		{Kind: "lens", Mode: "percent", Value: 101}:      false,
		{Kind: "frame", Mode: "delivery_fee"}:            true,
		{Kind: "frame", Mode: "none"}:                    true,
		{Kind: "frame", Mode: "full"}:                    false,
		{Kind: "frame", Mode: "fixed", Value: 0}:         false,
		{Kind: "frame", Mode: "delivery_fee", Value: 80}: false,
		{Kind: "both", Mode: "full"}:                     false,
	} {
		if validPolicy(p) != want {
			t.Fatalf("%+v: valid = %v", p, !want)
		}
	}
}

// TestSaveMethodNeedsANumberToTurnOn refuses turning bKash on without a valid receiving number.
func TestSaveMethodNeedsANumberToTurnOn(t *testing.T) {
	store := &storeStub{}
	service := Service{Store: store}
	for name, m := range map[string]Method{
		"on without number":     {Code: MethodBkash, Enabled: true},
		"bad number":            {Code: MethodBkash, AccountNumber: "12345"},
		"unknown method":        {Code: "nagad", AccountNumber: "01711111111"},
		"instructions too long": {Code: MethodBkash, Instructions: strings.Repeat("x", 1001)},
	} {
		if err := service.SaveMethod(context.Background(), m); !errors.Is(err, ErrInvalidPayment) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	if err := service.SaveMethod(context.Background(), Method{Code: MethodBkash, Enabled: true, AccountNumber: "+8801711111111"}); err != nil || store.saved[0].AccountNumber != "01711111111" {
		t.Fatalf("valid save = %+v %v", store.saved, err)
	}
}

// TestRejectNeedsAReason gives the customer something to act on.
func TestRejectNeedsAReason(t *testing.T) {
	service := Service{Store: &storeStub{}}
	if err := service.Reject(context.Background(), 1, 1, "   "); !errors.Is(err, ErrInvalidPayment) {
		t.Fatalf("empty reason: %v", err)
	}
	if err := service.Approve(context.Background(), 1, 0); !errors.Is(err, ErrInvalidPayment) {
		t.Fatalf("approval without an admin: %v", err)
	}
}
