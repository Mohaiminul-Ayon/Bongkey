ALTER TABLE products ADD COLUMN IF NOT EXISTS parent_product_id INTEGER REFERENCES products(id) ON DELETE RESTRICT;
ALTER TABLE products ADD COLUMN IF NOT EXISTS sku TEXT;
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS sku TEXT NOT NULL DEFAULT '';
UPDATE products SET sku = 'BK-' || id WHERE sku IS NULL OR sku = '';
ALTER TABLE products ALTER COLUMN sku SET DEFAULT ('BK-' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::text, '-', ''), 1, 12)));
ALTER TABLE products ALTER COLUMN sku SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS products_sku_unique ON products (LOWER(sku));
CREATE UNIQUE INDEX IF NOT EXISTS products_variant_option_unique ON products (COALESCE(parent_product_id, id), LOWER(TRIM(color)), LOWER(TRIM(size)));
CREATE INDEX IF NOT EXISTS products_parent_product_id_idx ON products (parent_product_id);
