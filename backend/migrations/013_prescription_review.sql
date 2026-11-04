-- The owner's private note on a prescription check, e.g. "left eye unclear, called customer". Admin-only.
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS prescription_note TEXT NOT NULL DEFAULT '' CHECK (char_length(prescription_note) <= 500);
