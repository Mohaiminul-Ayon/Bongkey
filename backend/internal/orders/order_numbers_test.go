package orders

import (
	"context"
	"errors"
	"testing"
	"time"
)

// emailOrders is an in-memory EmailOrderLister keyed by lower-case email.
type emailOrders map[string][]EmailedOrder

// OrdersByEmail returns the saved list for an email.
func (o emailOrders) OrdersByEmail(_ context.Context, email string, _ int) ([]EmailedOrder, error) {
	return o[email], nil
}

// sentNumbers records each reminder the service asked to queue.
type sentNumbers struct {
	to, key []string
	lists   [][]EmailedOrder
}

// OrderNumbers records one queued reminder.
func (s *sentNumbers) OrderNumbers(_ context.Context, to, key string, list []EmailedOrder) {
	s.to, s.key, s.lists = append(s.to, to), append(s.key, key), append(s.lists, list)
}

// TestSendOrderNumbers checks that only an email with orders gets a reminder, that the answer does not depend
// on whether orders exist, and that one client is rate limited.
func TestSendOrderNumbers(t *testing.T) {
	placed := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	sent := &sentNumbers{}
	service := OrderNumberService{
		Orders: emailOrders{"rahim@example.com": {{OrderNumber: "ORD-000123", PlacedAt: placed}}},
		Sender: sent, Limiter: countingLimiter{}, Secret: testEmailProofSecret,
	}
	ctx := context.Background()

	if err := service.SendOrderNumbers(ctx, "  Rahim@Example.com ", "203.0.113.1"); err != nil {
		t.Fatalf("known email: %v", err)
	}
	if len(sent.to) != 1 || sent.to[0] != "rahim@example.com" || sent.lists[0][0].OrderNumber != "ORD-000123" {
		t.Fatalf("reminder = %+v", sent)
	}
	if sent.key[0] == "" || sent.key[0] == "rahim@example.com" {
		t.Fatalf("queue key must be an HMAC, got %q", sent.key[0])
	}
	if err := service.SendOrderNumbers(ctx, "nobody@example.com", "203.0.113.1"); err != nil || len(sent.to) != 1 {
		t.Fatalf("unknown email: err=%v sent=%d", err, len(sent.to))
	}
	for _, bad := range []string{"", "abc", "Rahim <rahim@example.com>"} {
		if err := service.SendOrderNumbers(ctx, bad, "203.0.113.2"); !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("email %q: err=%v", bad, err)
		}
	}

	limited := OrderNumberService{Orders: emailOrders{}, Sender: &sentNumbers{}, Limiter: countingLimiter{}, Secret: testEmailProofSecret}
	for i := 1; i <= orderNumberRequestsPerWindow; i++ {
		if err := limited.SendOrderNumbers(ctx, "a@example.com", "198.51.100.9"); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	if err := limited.SendOrderNumbers(ctx, "a@example.com", "198.51.100.9"); !errors.Is(err, ErrTrackLimited) {
		t.Fatalf("over the limit: %v", err)
	}
	if err := (OrderNumberService{Orders: emailOrders{}, Sender: &sentNumbers{}}).SendOrderNumbers(ctx, "a@example.com", "198.51.100.9"); err == nil {
		t.Fatal("a missing limiter must fail closed")
	}
}
