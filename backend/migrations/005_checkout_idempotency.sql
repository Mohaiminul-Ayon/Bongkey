-- Hashes keep retry keys out of the database while the unique index arbitrates checkout ownership.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS idempotency_key_hash TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS idempotency_request_hash TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS orders_idempotency_key_hash_unique
    ON orders (idempotency_key_hash)
    WHERE idempotency_key_hash IS NOT NULL;
