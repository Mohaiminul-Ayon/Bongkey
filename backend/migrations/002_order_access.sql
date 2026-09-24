-- Existing orders intentionally receive no guest credential; identifiers never prove ownership.
CREATE TABLE IF NOT EXISTS order_access (
    order_id INTEGER PRIMARY KEY REFERENCES orders(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    expires_at TIMESTAMPTZ NOT NULL
);
