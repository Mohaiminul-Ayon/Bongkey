-- Staff-only note on an order ("called twice, no answer", "deliver after 5pm"). Customers never see it.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS staff_note TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'orders_staff_note_length') THEN
        ALTER TABLE orders ADD CONSTRAINT orders_staff_note_length CHECK (char_length(staff_note) <= 1000);
    END IF;
END $$;
