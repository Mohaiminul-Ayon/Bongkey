-- Call-back requests from the lens panel's custom lens step: the shopper describes their lens needs and
-- leaves a name and phone; staff call or chat with them. No order exists yet, so nothing here holds stock.
CREATE TABLE IF NOT EXISTS callback_requests (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 2 AND 80),
    phone TEXT NOT NULL CHECK (phone ~ '^01[3-9][0-9]{8}$'),
    -- The frame the shopper was looking at; empty when the request did not come from a product page.
    product_slug TEXT NOT NULL DEFAULT '' CHECK (product_slug = '' OR product_slug ~ '^[a-z0-9-]{1,160}$'),
    message TEXT NOT NULL CHECK (char_length(message) BETWEEN 5 AND 500),
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'done')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    handled_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS callback_requests_created_idx ON callback_requests (created_at DESC);
