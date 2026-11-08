-- Customer Google sign-in (#79). Customers are separate from admins: a customer session never opens admin routes.
-- google_sub is Google's stable account ID; the email is refreshed from Google on every sign-in.
CREATE TABLE IF NOT EXISTS customers (
    id BIGSERIAL PRIMARY KEY,
    google_sub TEXT NOT NULL UNIQUE CHECK (char_length(google_sub) BETWEEN 1 AND 255),
    email TEXT NOT NULL CHECK (char_length(email) BETWEEN 3 AND 254),
    name TEXT NOT NULL DEFAULT '' CHECK (char_length(name) <= 120),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Only the SHA-256 of the cookie token is stored, so a database leak does not hand out sessions.
CREATE TABLE IF NOT EXISTS customer_sessions (
    token_hash TEXT PRIMARY KEY,
    customer_id BIGINT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS customer_sessions_expires_idx ON customer_sessions (expires_at);

-- An order belongs to a customer only after an explicit, phone-checked link; guest orders keep NULL.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS customer_id BIGINT REFERENCES customers(id);
CREATE INDEX IF NOT EXISTS orders_customer_idx ON orders (customer_id) WHERE customer_id IS NOT NULL;
