package delivery

import (
	"context"
	"database/sql"
)

// PostgresRepository stores the singleton delivery setting in PostgreSQL.
type PostgresRepository struct{ DB *sql.DB }

// Get loads the current global delivery charge.
func (r PostgresRepository) Get(ctx context.Context) (Settings, error) {
	var settings Settings
	err := r.DB.QueryRowContext(ctx, `SELECT delivery_fee, outside_dhaka_fee FROM store_settings WHERE id = 1`).Scan(&settings.DeliveryFee, &settings.OutsideDhakaFee)
	return settings, err
}

// Update saves and returns the global delivery charge atomically.
func (r PostgresRepository) Update(ctx context.Context, fee, outsideFee int) (Settings, error) {
	var settings Settings
	err := r.DB.QueryRowContext(ctx, `UPDATE store_settings SET delivery_fee = $1, outside_dhaka_fee = $2 WHERE id = 1 RETURNING delivery_fee, outside_dhaka_fee`, fee, outsideFee).Scan(&settings.DeliveryFee, &settings.OutsideDhakaFee)
	return settings, err
}
