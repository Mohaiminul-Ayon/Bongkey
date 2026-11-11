-- Return and exchange tickets (owner rules 2026-11-12): after delivery a customer asks for a frame exchange
-- (3 days) or reports a defect, wrong colour or wrong item (15 days). Staff call, agree a solution and close
-- the ticket; a refund (problems only) is recorded with its real payment details, which is not the same as
-- agreeing to refund.
CREATE TABLE IF NOT EXISTS return_requests (
    id BIGSERIAL PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('exchange', 'defect', 'wrong_item')),
    message TEXT NOT NULL CHECK (char_length(message) BETWEEN 5 AND 1000),
    -- Random file names in the private photo folder; never served publicly.
    photos TEXT[] NOT NULL DEFAULT '{}' CHECK (cardinality(photos) <= 3),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'closed')),
    outcome TEXT NOT NULL DEFAULT '' CHECK (outcome IN ('', 'exchanged', 'replaced', 'refunded', 'declined')),
    staff_note TEXT NOT NULL DEFAULT '' CHECK (char_length(staff_note) <= 1000),
    refund_amount INTEGER CHECK (refund_amount IS NULL OR refund_amount > 0),
    refund_method TEXT NOT NULL DEFAULT '' CHECK (refund_method IN ('', 'bkash', 'cash')),
    refund_reference TEXT NOT NULL DEFAULT '' CHECK (char_length(refund_reference) <= 60),
    refunded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ,
    -- A closed ticket always says how it ended, and only a closed one does.
    CHECK ((status = 'closed') = (outcome <> '')),
    -- "Refunded" needs the money actually sent: amount, method and date; only problem tickets are refunded.
    CHECK ((outcome = 'refunded') = (refund_amount IS NOT NULL AND refund_method <> '' AND refunded_at IS NOT NULL)),
    CHECK (outcome <> 'refunded' OR kind <> 'exchange')
);

-- One open ticket per order, so a double submit or a second device cannot open two at once.
CREATE UNIQUE INDEX IF NOT EXISTS return_requests_one_open ON return_requests (order_id) WHERE status <> 'closed';
CREATE INDEX IF NOT EXISTS return_requests_created_idx ON return_requests (created_at DESC);

-- Staff decisions and refunds are money changes, so they go to the audit log (022).
DROP TRIGGER IF EXISTS audit_return_requests ON return_requests;
CREATE TRIGGER audit_return_requests AFTER UPDATE ON return_requests
    FOR EACH ROW EXECUTE FUNCTION audit_change('id', 'status', 'outcome', 'refund_amount', 'refund_method', 'refund_reference');
