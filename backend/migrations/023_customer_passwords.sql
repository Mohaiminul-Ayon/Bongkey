-- Email + password customer accounts and the required profile (name + mobile).
-- A customer may now have Google, a password, or both, so google_sub becomes optional.
ALTER TABLE customers ALTER COLUMN google_sub DROP NOT NULL;

-- bcrypt hash only; NULL means the customer has never set a password (Google only).
ALTER TABLE customers ADD COLUMN IF NOT EXISTS password_hash TEXT CHECK (password_hash IS NULL OR char_length(password_hash) BETWEEN 50 AND 100);

-- Stored as "01XXXXXXXXX"; empty until the customer finishes the profile. Not unique: family members may share a phone.
ALTER TABLE customers ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT '' CHECK (phone = '' OR phone ~ '^01[3-9][0-9]{8}$');

-- One account per email, so a Google sign-in and a password sign-up for the same address reach the same orders.
CREATE UNIQUE INDEX IF NOT EXISTS customers_email_unique ON customers (LOWER(email));
