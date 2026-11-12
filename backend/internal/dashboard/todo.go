// Package dashboard reads the staff "what needs me now" counts across orders, payments, call-backs and returns.
// It only counts rows; each feature package still owns its own rules and changes.
package dashboard

import (
	"context"
	"database/sql"
	"time"

	"bongkey/backend/internal/returns"
)

// Todo is the number of items waiting for staff in each queue.
type Todo struct {
	OrdersToConfirm int `json:"orders_to_confirm"`
	PaymentsToCheck int `json:"payments_to_check"`
	CallbacksOpen   int `json:"callbacks_open"`
	ReturnsNew      int `json:"returns_new"`
	// ReturnsOverdue counts open problem tickets past the refund deadline, whether new or in progress.
	ReturnsOverdue int `json:"returns_overdue"`
}

// Reader counts the waiting work in one query.
type Reader struct{ DB *sql.DB }

// Todo counts every waiting item in the database (not a page of recent rows), so busy days are counted correctly.
func (r Reader) Todo(ctx context.Context) (Todo, error) {
	// The overdue cutoff comes from the returns package, so the badge and the Returns page agree.
	overdueBefore := time.Now().Add(-returns.RefundDeadline)
	var todo Todo
	err := r.DB.QueryRowContext(ctx, `
		SELECT
			(SELECT count(*) FROM orders WHERE order_status = 'pending'),
			(SELECT count(*) FROM payments WHERE status = 'pending'),
			(SELECT count(*) FROM callback_requests WHERE status = 'open'),
			(SELECT count(*) FROM return_requests WHERE status = 'open'),
			(SELECT count(*) FROM return_requests WHERE status <> 'closed' AND kind <> 'exchange' AND created_at < $1)
	`, overdueBefore).Scan(&todo.OrdersToConfirm, &todo.PaymentsToCheck, &todo.CallbacksOpen, &todo.ReturnsNew, &todo.ReturnsOverdue)
	return todo, err
}
