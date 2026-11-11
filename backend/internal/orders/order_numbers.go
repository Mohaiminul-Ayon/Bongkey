package orders

import (
	"context"
	"errors"
	"strings"
	"time"

	"bongkey/backend/internal/security"
)

// orderNumberRequestsPerWindow caps how many emails one client can ask for in 15 minutes, so the form
// cannot be used to flood many inboxes.
const orderNumberRequestsPerWindow = 5

// maxEmailedOrders bounds one reminder email to the most recent orders.
const maxEmailedOrders = 10

// EmailedOrder is what the reminder email shows: the order number and the day it was placed, nothing private.
type EmailedOrder struct {
	OrderNumber string
	PlacedAt    time.Time
}

// OrderNumberSender queues the reminder email; notifications.Service implements it.
type OrderNumberSender interface {
	OrderNumbers(ctx context.Context, to, emailKey string, list []EmailedOrder)
}

// EmailOrderLister finds the orders placed with one email.
type EmailOrderLister interface {
	OrdersByEmail(ctx context.Context, email string, limit int) ([]EmailedOrder, error)
}

// OrderNumberService emails a guest the order numbers placed with their email. No code is needed: the
// answer goes only to that inbox, so only its owner can read it.
type OrderNumberService struct {
	Orders  EmailOrderLister
	Sender  OrderNumberSender
	Limiter AttemptCounter
	// Secret keys the HMACs that hide the client IP and email in limiter and queue keys.
	Secret string
}

// SendOrderNumbers queues the reminder when the email has orders. The caller always shows the same answer,
// so the form never reveals whether an email was used at checkout.
func (s OrderNumberService) SendOrderNumbers(ctx context.Context, email, clientIP string) error {
	if s.Limiter == nil || clientIP == "" {
		return errors.New("order number limiter is not configured")
	}
	attempts, err := s.Limiter.Count(ctx, security.EmailKey(s.Secret, "order-numbers-ip", clientIP))
	if err != nil {
		return err
	}
	if attempts > orderNumberRequestsPerWindow {
		return ErrTrackLimited
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !plainEmail(email) {
		return ErrInvalidEmail
	}
	list, err := s.Orders.OrdersByEmail(ctx, email, maxEmailedOrders)
	if err != nil || len(list) == 0 {
		return err
	}
	s.Sender.OrderNumbers(ctx, email, security.EmailKey(s.Secret, "order-numbers-email", email), list)
	return nil
}

// OrdersByEmail lists the newest orders whose checkout email matches, ignoring case and spaces.
func (r PostgresReader) OrdersByEmail(ctx context.Context, email string, limit int) ([]EmailedOrder, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT order_number, created_at FROM orders
		WHERE LOWER(TRIM(customer_email)) = $1
		ORDER BY created_at DESC LIMIT $2`, email, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []EmailedOrder
	for rows.Next() {
		var order EmailedOrder
		if err := rows.Scan(&order.OrderNumber, &order.PlacedAt); err != nil {
			return nil, err
		}
		list = append(list, order)
	}
	return list, rows.Err()
}
