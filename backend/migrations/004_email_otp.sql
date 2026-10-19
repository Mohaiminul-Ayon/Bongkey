-- Existing order emails were never verified and cannot authorize private access.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS email_otp_challenges (
    email_key TEXT PRIMARY KEY,
    code_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    next_request_at TIMESTAMPTZ NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    send_count INTEGER NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    consumed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS email_otp_ip_limits (
    ip_key TEXT PRIMARY KEY,
    window_start TIMESTAMPTZ NOT NULL,
    request_count INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS email_otp_challenges_window_idx ON email_otp_challenges (window_start);
CREATE INDEX IF NOT EXISTS email_otp_ip_limits_window_idx ON email_otp_ip_limits (window_start);
