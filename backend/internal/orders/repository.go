package orders

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
)

// PostgresReader reads order details and the guest access hash from PostgreSQL.
type PostgresReader struct {
	DB *sql.DB
}

// AdminOrder is the summary shown in the owner order list.
type AdminOrder struct {
	ID            int    `json:"id"`
	OrderNumber   string `json:"order_number"`
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	City          string `json:"city"`
	Total         int    `json:"total"`
	OrderStatus   string `json:"order_status"`
	CreatedAt     string `json:"created_at"`
	// RiskFlags are the red-flag codes from checkout; staff look twice at these orders.
	RiskFlags []string `json:"risk_flags"`
}

// AdminOrderPageSize bounds one admin order list response so the page stays fast as orders grow.
const AdminOrderPageSize = 50

// ErrInvalidOrderFilter means an admin order filter value cannot be used.
var ErrInvalidOrderFilter = errors.New("invalid order filter")

// maxOrderSearchLength bounds search text; order numbers and phones are far shorter.
const maxOrderSearchLength = 50

// shopZone decides which orders belong to a calendar day; the shop runs on Bangladesh time.
var shopZone = time.FixedZone("BDT", 6*60*60)

// AdminOrderFilter narrows the owner order list. Empty fields do not filter.
type AdminOrderFilter struct {
	Search string
	Status string
	From   *time.Time // first instant included
	Until  *time.Time // first instant excluded
}

// NewAdminOrderFilter validates raw filter text; from and to are inclusive YYYY-MM-DD days.
func NewAdminOrderFilter(search, status, from, to string) (AdminOrderFilter, error) {
	filter := AdminOrderFilter{Search: strings.TrimSpace(search), Status: status}
	if utf8.RuneCountInString(filter.Search) > maxOrderSearchLength || (status != "" && !knownOrderStatus(status)) {
		return AdminOrderFilter{}, ErrInvalidOrderFilter
	}
	if from != "" {
		day, err := time.ParseInLocation("2006-01-02", from, shopZone)
		if err != nil {
			return AdminOrderFilter{}, ErrInvalidOrderFilter
		}
		filter.From = &day
	}
	if to != "" {
		day, err := time.ParseInLocation("2006-01-02", to, shopZone)
		if err != nil || (filter.From != nil && day.Before(*filter.From)) {
			return AdminOrderFilter{}, ErrInvalidOrderFilter
		}
		nextDay := day.AddDate(0, 0, 1)
		filter.Until = &nextDay
	}
	return filter, nil
}

// ListAdmin returns one filtered page of order summaries older than beforeID (0 means newest), without guest access hashes.
// Keyset paging on the primary key stays stable while new orders arrive, unlike OFFSET.
// Search matches part of the order number or phone; position() keeps % and _ literal, unlike LIKE.
func (r PostgresReader) ListAdmin(ctx context.Context, beforeID int, filter AdminOrderFilter) ([]AdminOrder, error) {
	if r.DB == nil {
		return nil, errors.New("order storage unavailable")
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, order_number, customer_name, customer_phone, city, total, order_status, created_at, risk_flags
		FROM orders
		WHERE ($1 = 0 OR id < $1)
			AND ($3 = '' OR position(lower($3) IN lower(order_number)) > 0 OR position($3 IN customer_phone) > 0)
			AND ($4 = '' OR order_status = $4)
			AND ($5::timestamptz IS NULL OR created_at >= $5)
			AND ($6::timestamptz IS NULL OR created_at < $6)
		ORDER BY id DESC LIMIT $2
	`, beforeID, AdminOrderPageSize, filter.Search, filter.Status, filter.From, filter.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]AdminOrder, 0)
	for rows.Next() {
		var order AdminOrder
		if err := rows.Scan(&order.ID, &order.OrderNumber, &order.CustomerName, &order.CustomerPhone, &order.City, &order.Total, &order.OrderStatus, &order.CreatedAt, pq.Array(&order.RiskFlags)); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

// findOrderSQL is shared by both lookups; only the fixed WHERE clause differs, never user text.
const findOrderSQL = `
	SELECT o.id, o.order_number, o.customer_name, o.customer_phone, o.shipping_address,
		o.city, o.subtotal, o.delivery_fee, o.total, o.order_status,
		COALESCE(i.invoice_number, ''), o.guest_token_hash, o.customer_email, o.email_verified,
		o.delivery_note, o.payment_method, o.payment_status, o.created_at,
		COALESCE(i.issued_at::text, ''),
		EXISTS (SELECT 1 FROM order_items x WHERE x.order_id = o.id AND NOT x.lens_price_final),
		EXISTS (SELECT 1 FROM order_items x WHERE x.order_id = o.id AND x.lens_kind <> 'none'),
		o.advance_mode, o.advance_required, o.amount_paid,
		o.created_at, o.confirmed_at, o.preparing_at, o.shipped_at, o.delivered_at, o.cancelled_at,
		o.courier_name, o.tracking_code, o.tracking_url, o.email_confirmed, o.risk_flags
	FROM orders o LEFT JOIN invoices i ON i.order_id = o.id
	WHERE `

// Find loads one order by numeric ID or order number; missing rows become the same error as denied guest access.
func (r PostgresReader) Find(ctx context.Context, id string) (Record, error) {
	// Comparing the typed key keeps the primary-key and unique order_number indexes usable; id::text forced a scan.
	query, arg := findOrderSQL+"o.order_number = $1", any(id)
	if numericID, err := strconv.Atoi(id); err == nil {
		if numericID <= 0 {
			return Record{}, ErrNotFound
		}
		query, arg = findOrderSQL+"o.id = $1", numericID
	}
	var record Record
	err := r.DB.QueryRowContext(ctx, query, arg).Scan(
		&record.ID, &record.OrderNumber, &record.CustomerName, &record.CustomerPhone,
		&record.ShippingAddress, &record.City, &record.Subtotal, &record.DeliveryFee,
		&record.Total, &record.Status, &record.InvoiceNumber, &record.GuestTokenHash,
		&record.CustomerEmail, &record.EmailVerified, &record.DeliveryNote,
		&record.PaymentMethod, &record.PaymentStatus, &record.CreatedAt, &record.InvoiceIssuedAt, &record.LensPricePending,
		&record.LensOrder, &record.AdvanceMode, &record.AdvanceRequired, &record.AmountPaid,
		&record.Tracking.PlacedAt, &record.Tracking.ConfirmedAt, &record.Tracking.PreparingAt, &record.Tracking.ShippedAt,
		&record.Tracking.DeliveredAt, &record.Tracking.CancelledAt,
		&record.Tracking.Courier, &record.Tracking.TrackingCode, &record.Tracking.TrackingURL, &record.EmailConfirmed, pq.Array(&record.RiskFlags),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	record.PayInAdvance = record.AdvanceRequired > 0
	return record, nil
}

// Items loads immutable line snapshots for a previously authorized admin order.
func (r PostgresReader) Items(ctx context.Context, orderID int) ([]Item, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, product_name, sku, lens_option, quantity, unit_price, total_price,
			lens_kind, lens_name, lens_power, sph_right, sph_left, lens_price, lens_price_final,
			prescription_status, prescription_file <> '', prescription_note, lens_request
		FROM order_items WHERE order_id = $1 ORDER BY id
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Item, 0)
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.ProductName, &item.SKU, &item.LensOption, &item.Quantity, &item.UnitPrice, &item.TotalPrice,
			&item.LensKind, &item.LensName, &item.LensPower, &item.SPHRight, &item.SPHLeft, &item.LensPrice, &item.LensPriceFinal,
			&item.PrescriptionStatus, &item.HasPrescription, &item.PrescriptionNote, &item.LensRequest); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
