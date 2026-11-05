-- Advance rules (#19): what a customer pays by bKash before the order is confirmed.
-- The owner edits one rule for frame-only orders and one for orders with lenses; each order keeps a copy
-- of the rule it was placed under, so later setting changes never alter an existing agreement.

CREATE TABLE IF NOT EXISTS advance_policies (
    kind TEXT PRIMARY KEY CHECK (kind IN ('frame', 'lens')),
    mode TEXT NOT NULL CHECK (mode IN ('none', 'delivery_fee', 'percent', 'fixed', 'full')),
    value INTEGER NOT NULL DEFAULT 0 CHECK (value >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Lenses are made to measure, so a lens order always pays something in advance.
    CHECK (kind <> 'lens' OR mode IN ('percent', 'fixed', 'full')),
    -- "Full" for a frame order is the customer's own bKash choice, not an owner rule.
    CHECK (kind <> 'frame' OR mode IN ('none', 'delivery_fee', 'percent', 'fixed')),
    CHECK (mode <> 'percent' OR value BETWEEN 1 AND 100),
    CHECK (mode <> 'fixed' OR value > 0)
);

-- Owner decision 2026-11-05: lens orders pay in full; cash-on-delivery frame orders pay the delivery fee first.
INSERT INTO advance_policies (kind, mode) VALUES ('frame', 'delivery_fee'), ('lens', 'full') ON CONFLICT (kind) DO NOTHING;

ALTER TABLE orders ADD COLUMN IF NOT EXISTS advance_mode TEXT NOT NULL DEFAULT 'none'
    CHECK (advance_mode IN ('none', 'delivery_fee', 'percent', 'fixed', 'full'));
ALTER TABLE orders ADD COLUMN IF NOT EXISTS advance_value INTEGER NOT NULL DEFAULT 0 CHECK (advance_value >= 0);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS advance_required INTEGER NOT NULL DEFAULT 0 CHECK (advance_required >= 0);
ALTER TABLE orders ADD COLUMN IF NOT EXISTS amount_paid INTEGER NOT NULL DEFAULT 0 CHECK (amount_paid >= 0);

-- Orders placed before this rule existed: every "advance" order was a full bKash payment or a lens order
-- (local test data only; no production orders existed). Runs once, because new advance orders never keep mode 'none'.
UPDATE orders SET advance_mode = 'full', advance_required = total
WHERE payment_method = 'advance' AND advance_mode = 'none';

-- Approved bKash payments from #18 become the order's recorded paid amount.
UPDATE orders o SET amount_paid = p.sum
FROM (SELECT order_id, SUM(amount)::INTEGER AS sum FROM payments WHERE status = 'approved' GROUP BY order_id) p
WHERE o.id = p.order_id AND o.amount_paid = 0;
