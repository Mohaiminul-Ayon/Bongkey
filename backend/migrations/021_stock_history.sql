-- Stock alerts and history (#20). Existing stock values are not touched.
-- 0 means "no alert"; the owner sets a number per frame or option.
ALTER TABLE products ADD COLUMN IF NOT EXISTS low_stock_threshold INTEGER NOT NULL DEFAULT 0
    CHECK (low_stock_threshold BETWEEN 0 AND 100000);

-- One row per stock change, written by the trigger below in the same transaction as the change,
-- so no code path (checkout, cancellation, admin edit, new option) can change stock without a record.
CREATE TABLE IF NOT EXISTS stock_history (
    id BIGSERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    change INTEGER NOT NULL,
    stock_after INTEGER NOT NULL,
    reason TEXT NOT NULL,
    order_id INTEGER REFERENCES orders(id) ON DELETE SET NULL,
    -- tx_id lets checkout attach its order number after the stock was reserved earlier in the same transaction.
    tx_id BIGINT NOT NULL DEFAULT txid_current(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS stock_history_product_idx ON stock_history (product_id, id DESC);

-- The writer labels a change with SET LOCAL-style settings (set_config(..., true)):
-- bongkey.stock_reason (order_placed, order_cancelled, admin_edit) and bongkey.stock_order (an order id).
-- An unlabelled change, such as a hand-written SQL fix, is recorded as "unlabelled".
CREATE OR REPLACE FUNCTION record_stock_change() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.stock_quantity <> 0 THEN
            INSERT INTO stock_history (product_id, change, stock_after, reason)
            VALUES (NEW.id, NEW.stock_quantity, NEW.stock_quantity, 'created');
        END IF;
    ELSIF NEW.stock_quantity IS DISTINCT FROM OLD.stock_quantity THEN
        INSERT INTO stock_history (product_id, change, stock_after, reason, order_id)
        VALUES (NEW.id, NEW.stock_quantity - OLD.stock_quantity, NEW.stock_quantity,
            COALESCE(NULLIF(current_setting('bongkey.stock_reason', true), ''), 'unlabelled'),
            NULLIF(current_setting('bongkey.stock_order', true), '')::INTEGER);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS products_stock_history ON products;
CREATE TRIGGER products_stock_history AFTER INSERT OR UPDATE OF stock_quantity ON products
    FOR EACH ROW EXECUTE FUNCTION record_stock_change();
