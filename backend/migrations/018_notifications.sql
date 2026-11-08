-- Each email is saved before it is sent, so a provider failure never blocks an order and a retry never sends twice.
-- event_key names the business event (for example order_placed:160); the unique key makes enqueueing idempotent.
CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    event_key TEXT NOT NULL UNIQUE CHECK (char_length(event_key) BETWEEN 1 AND 120),
    recipient TEXT NOT NULL CHECK (char_length(recipient) <= 254),
    subject TEXT NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 200),
    body TEXT NOT NULL,
    -- When set, the order's invoice PDF is rendered at send time and attached.
    invoice_order_id INTEGER REFERENCES orders(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'skipped')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at TIMESTAMPTZ,
    -- A short provider stage such as "status 401"; never an address or message text.
    last_error TEXT NOT NULL DEFAULT '' CHECK (char_length(last_error) <= 200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS notifications_due_idx ON notifications (next_attempt_at) WHERE status IN ('pending', 'sending');

-- Staff tick this after reading the email address back to the customer on the confirmation call;
-- the invoice email waits for it, because the call proves the phone, not the email.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS email_confirmed BOOLEAN NOT NULL DEFAULT false;
