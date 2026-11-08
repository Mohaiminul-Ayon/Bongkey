-- Red-flag codes set at checkout (for example short_address, repeat_phone); staff see them in red and look twice.
-- They never block an order.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS risk_flags TEXT[] NOT NULL DEFAULT '{}';
