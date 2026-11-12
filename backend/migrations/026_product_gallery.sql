-- Extra product photos shown after the cover photo on the product page. Each colour/size variant is its own
-- products row, so each colour keeps its own list. Order in the array is the display order.
ALTER TABLE products ADD COLUMN IF NOT EXISTS gallery_urls TEXT[] NOT NULL DEFAULT '{}';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'products_gallery_urls_max') THEN
        ALTER TABLE products ADD CONSTRAINT products_gallery_urls_max CHECK (cardinality(gallery_urls) <= 6);
    END IF;
END $$;
