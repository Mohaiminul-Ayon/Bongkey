-- Keep the authoritative product stock nonnegative even for writes outside checkout.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'products'::regclass
          AND conname = 'products_stock_quantity_nonnegative'
    ) THEN
        ALTER TABLE products
            ADD CONSTRAINT products_stock_quantity_nonnegative
            CHECK (stock_quantity >= 0);
    END IF;
END $$;
