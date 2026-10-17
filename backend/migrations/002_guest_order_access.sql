-- Existing orders have no guest token and remain inaccessible to guest lookup.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS guest_token_hash TEXT NOT NULL DEFAULT '';
