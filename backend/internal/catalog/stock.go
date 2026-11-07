package catalog

import (
	"context"
	"time"
)

// maxLowStockThreshold matches the database check on products.low_stock_threshold.
const maxLowStockThreshold = 100000

// LowStockItem is one frame or option at or below its alert level.
type LowStockItem struct {
	ID        int    `json:"id"`
	RootID    int    `json:"root_id"`
	Name      string `json:"name"`
	SKU       string `json:"sku"`
	Color     string `json:"color"`
	Size      string `json:"size"`
	Stock     int    `json:"stock_quantity"`
	Threshold int    `json:"low_stock_threshold"`
}

// StockChange is one row of an item's stock history.
type StockChange struct {
	Change      int       `json:"change"`
	StockAfter  int       `json:"stock_after"`
	Reason      string    `json:"reason"`
	OrderNumber string    `json:"order_number"`
	CreatedAt   time.Time `json:"created_at"`
}

// StockStore reads stock alerts and history; the history itself is written by a database trigger.
type StockStore interface {
	LowStock(ctx context.Context) ([]LowStockItem, error)
	StockHistory(ctx context.Context, productID int) ([]StockChange, error)
}

// StockService serves the owner's low-stock list and stock history.
type StockService struct{ Store StockStore }

// LowStock lists active items whose stock fell to their alert level, lowest first.
func (s StockService) LowStock(ctx context.Context) ([]LowStockItem, error) {
	return s.Store.LowStock(ctx)
}

// History lists one item's latest stock changes, newest first.
func (s StockService) History(ctx context.Context, productID int) ([]StockChange, error) {
	if productID <= 0 {
		return nil, ErrNotFound
	}
	return s.Store.StockHistory(ctx, productID)
}

// LowStock reads items with an alert set (threshold > 0) whose stock is at or below it.
// shortcut: capped at 200 rows; a shop with more low items needs paging.
func (r PostgresProductRepository) LowStock(ctx context.Context) ([]LowStockItem, error) {
	if r.DB == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id, COALESCE(parent_product_id, id), name, sku, color, size, stock_quantity, low_stock_threshold
		FROM products
		WHERE is_active AND low_stock_threshold > 0 AND stock_quantity <= low_stock_threshold
		ORDER BY stock_quantity, id LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []LowStockItem{}
	for rows.Next() {
		var item LowStockItem
		if err := rows.Scan(&item.ID, &item.RootID, &item.Name, &item.SKU, &item.Color, &item.Size, &item.Stock, &item.Threshold); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// StockHistory reads the latest 100 changes for one item, with the order number when an order caused it.
func (r PostgresProductRepository) StockHistory(ctx context.Context, productID int) ([]StockChange, error) {
	if r.DB == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT h.change, h.stock_after, h.reason, COALESCE(o.order_number, ''), h.created_at
		FROM stock_history h LEFT JOIN orders o ON o.id = h.order_id
		WHERE h.product_id = $1 ORDER BY h.id DESC LIMIT 100`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	changes := []StockChange{}
	for rows.Next() {
		var change StockChange
		if err := rows.Scan(&change.Change, &change.StockAfter, &change.Reason, &change.OrderNumber, &change.CreatedAt); err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, rows.Err()
}
