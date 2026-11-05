-- Delivery tracking (#21): when each status step happened, and the courier details the owner types in.
-- Times are set by the status change itself, once, because the delivered time starts the return window.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS confirmed_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS preparing_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipped_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS courier_name TEXT NOT NULL DEFAULT '' CHECK (char_length(courier_name) <= 60);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS tracking_code TEXT NOT NULL DEFAULT '' CHECK (char_length(tracking_code) <= 80);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS tracking_url TEXT NOT NULL DEFAULT ''
    CHECK (tracking_url = '' OR (tracking_url LIKE 'https://%' AND char_length(tracking_url) <= 300));

-- An order already shipped before this migration was last changed when it shipped, so that is its ship time;
-- without it, marking it delivered would break the rule below. Runs once: later shipped orders have a time.
UPDATE orders SET shipped_at = updated_at WHERE order_status = 'shipped' AND shipped_at IS NULL;

-- Delivered can never come before shipped. Older delivered orders have no times and pass.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'orders'::regclass
          AND conname = 'orders_delivered_after_shipped'
    ) THEN
        ALTER TABLE orders
            ADD CONSTRAINT orders_delivered_after_shipped
            CHECK (delivered_at IS NULL OR (shipped_at IS NOT NULL AND delivered_at >= shipped_at));
    END IF;
END $$;
