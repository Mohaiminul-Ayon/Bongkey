ALTER TABLE store_settings
ADD COLUMN IF NOT EXISTS outside_dhaka_fee INTEGER NOT NULL DEFAULT 130 CHECK (outside_dhaka_fee >= 0);
