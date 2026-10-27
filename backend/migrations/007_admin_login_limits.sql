-- Login attempts are keyed by an HMAC of the client IP so the table never stores raw addresses.
CREATE TABLE IF NOT EXISTS admin_login_limits (
    client_key TEXT PRIMARY KEY,
    window_start TIMESTAMPTZ NOT NULL,
    attempt_count INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS admin_login_limits_window_idx ON admin_login_limits (window_start);
