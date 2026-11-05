-- Manual mobile payments: the customer sends money outside the site and submits the transaction ID;
-- nothing counts as paid until the owner approves it.

-- One row per method the shop can receive with; the owner edits the number and instructions in admin.
CREATE TABLE IF NOT EXISTS payment_methods (
    code TEXT PRIMARY KEY CHECK (code IN ('bkash')),
    enabled BOOLEAN NOT NULL DEFAULT false,
    account_number TEXT NOT NULL DEFAULT '' CHECK (account_number = '' OR account_number ~ '^01[3-9][0-9]{8}$'),
    instructions TEXT NOT NULL DEFAULT '' CHECK (char_length(instructions) <= 1000),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO payment_methods (code) VALUES ('bkash') ON CONFLICT (code) DO NOTHING;

-- Every submission is kept, including rejected ones, so the review history stays visible.
CREATE TABLE IF NOT EXISTS payments (
    id SERIAL PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    method TEXT NOT NULL REFERENCES payment_methods(code),
    trx_id TEXT NOT NULL CHECK (trx_id ~ '^[A-Z0-9]{8,12}$'),
    sender_number TEXT NOT NULL CHECK (sender_number ~ '^01[3-9][0-9]{8}$'),
    amount INTEGER NOT NULL CHECK (amount > 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reject_reason TEXT NOT NULL DEFAULT '' CHECK (char_length(reject_reason) <= 200),
    reviewed_by INTEGER REFERENCES admins(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((status = 'pending') = (reviewed_at IS NULL)),
    CHECK (status <> 'rejected' OR reject_reason <> '')
);
-- A repeated submission of the same transaction on the same order is the same payment, unless it was rejected.
CREATE UNIQUE INDEX IF NOT EXISTS payments_order_trx_open_unique ON payments (order_id, method, trx_id) WHERE status <> 'rejected';
-- Finds the same transaction ID used on other orders, which the owner sees as a warning.
CREATE INDEX IF NOT EXISTS payments_method_trx_idx ON payments (method, trx_id);
CREATE INDEX IF NOT EXISTS payments_order_idx ON payments (order_id);
CREATE INDEX IF NOT EXISTS payments_pending_idx ON payments (created_at) WHERE status = 'pending';
