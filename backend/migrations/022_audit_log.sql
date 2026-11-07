-- Admin audit log (#26): who-changed-what-and-when for money and policy settings.
-- Triggers write each entry in the same transaction as the change, so a failed change leaves no entry,
-- and no code path can skip it. Only the listed columns are recorded: no customer contact data, tokens or files.
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    table_name TEXT NOT NULL,
    record_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('insert', 'update', 'delete')),
    changes JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS audit_log_created_idx ON audit_log (id DESC);

-- audit_change(key_column, watched_column...) records the watched columns that changed as {"col": {"from": x, "to": y}}.
CREATE OR REPLACE FUNCTION audit_change() RETURNS trigger AS $$
DECLARE
    old_row JSONB := CASE WHEN TG_OP <> 'INSERT' THEN to_jsonb(OLD) END;
    new_row JSONB := CASE WHEN TG_OP <> 'DELETE' THEN to_jsonb(NEW) END;
    changed JSONB := '{}';
    col TEXT;
BEGIN
    FOR i IN 1 .. TG_NARGS - 1 LOOP
        col := TG_ARGV[i];
        IF (old_row -> col) IS DISTINCT FROM (new_row -> col) THEN
            changed := changed || jsonb_build_object(col, jsonb_build_object('from', old_row -> col, 'to', new_row -> col));
        END IF;
    END LOOP;
    IF changed <> '{}' THEN
        INSERT INTO audit_log (table_name, record_id, action, changes)
        VALUES (TG_TABLE_NAME, COALESCE(new_row, old_row) ->> TG_ARGV[0], lower(TG_OP), changed);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- The log is append-only: the app has no edit path, and the database refuses one too.
CREATE OR REPLACE FUNCTION audit_log_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only';
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS audit_log_no_edit ON audit_log;
CREATE TRIGGER audit_log_no_edit BEFORE UPDATE OR DELETE ON audit_log FOR EACH ROW EXECUTE FUNCTION audit_log_append_only();

-- Prices, offers (old price) and publishing of frames; stock has its own history (021).
DROP TRIGGER IF EXISTS audit_products ON products;
CREATE TRIGGER audit_products AFTER INSERT OR UPDATE OR DELETE ON products
    FOR EACH ROW EXECUTE FUNCTION audit_change('id', 'price', 'old_price', 'is_active');
-- Delivery charges.
DROP TRIGGER IF EXISTS audit_store_settings ON store_settings;
CREATE TRIGGER audit_store_settings AFTER UPDATE ON store_settings
    FOR EACH ROW EXECUTE FUNCTION audit_change('id', 'delivery_fee', 'outside_dhaka_fee');
-- The bKash receiving number: a silent change would send customers' money elsewhere.
DROP TRIGGER IF EXISTS audit_payment_methods ON payment_methods;
CREATE TRIGGER audit_payment_methods AFTER UPDATE ON payment_methods
    FOR EACH ROW EXECUTE FUNCTION audit_change('code', 'enabled', 'account_number', 'instructions');
DROP TRIGGER IF EXISTS audit_advance_policies ON advance_policies;
CREATE TRIGGER audit_advance_policies AFTER UPDATE ON advance_policies
    FOR EACH ROW EXECUTE FUNCTION audit_change('kind', 'mode', 'value');
DROP TRIGGER IF EXISTS audit_lens_packages ON lens_packages;
CREATE TRIGGER audit_lens_packages AFTER INSERT OR UPDATE OR DELETE ON lens_packages
    FOR EACH ROW EXECUTE FUNCTION audit_change('id', 'price', 'active');
-- Payment approval or rejection (customer submissions are inserts and not logged).
DROP TRIGGER IF EXISTS audit_payments ON payments;
CREATE TRIGGER audit_payments AFTER UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION audit_change('id', 'status', 'amount', 'reject_reason');
-- Order status, payment state, paid amount and total (a lens price change moves the total).
DROP TRIGGER IF EXISTS audit_orders ON orders;
CREATE TRIGGER audit_orders AFTER UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION audit_change('id', 'order_status', 'payment_status', 'amount_paid', 'total');
DROP TRIGGER IF EXISTS audit_order_items ON order_items;
CREATE TRIGGER audit_order_items AFTER UPDATE ON order_items
    FOR EACH ROW EXECUTE FUNCTION audit_change('id', 'lens_price');
