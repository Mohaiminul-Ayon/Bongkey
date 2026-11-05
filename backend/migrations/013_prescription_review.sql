-- The owner's private note on a prescription check, e.g. "left eye unclear, called customer". Admin-only.
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS prescription_note TEXT NOT NULL DEFAULT '' CHECK (char_length(prescription_note) <= 500);
-- The customer's own description of a custom lens, written on the product page. Admin-only, never on the invoice.
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS lens_request TEXT NOT NULL DEFAULT '' CHECK (char_length(lens_request) <= 500);
