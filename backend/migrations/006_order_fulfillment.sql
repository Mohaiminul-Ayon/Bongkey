-- Bound fulfillment values so writes outside the service cannot invent states.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'orders'::regclass
          AND conname = 'orders_fulfillment_status_allowed'
    ) THEN
        ALTER TABLE orders
            ADD CONSTRAINT orders_fulfillment_status_allowed
            CHECK (order_status IN ('pending', 'confirmed', 'preparing', 'shipped', 'delivered', 'cancelled'))
            NOT VALID;
    END IF;
END $$;

ALTER TABLE orders VALIDATE CONSTRAINT orders_fulfillment_status_allowed;
