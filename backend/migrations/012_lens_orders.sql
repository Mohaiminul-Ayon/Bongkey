-- Powers are stored in hundredths of a diopter: 600 means ±6.00.
ALTER TABLE lens_packages ADD COLUMN IF NOT EXISTS max_power INTEGER NOT NULL DEFAULT 600 CHECK (max_power BETWEEN 25 AND 2000);
ALTER TABLE lens_packages ADD COLUMN IF NOT EXISTS prescription_required BOOLEAN NOT NULL DEFAULT false;

-- Each order line snapshots its lens choice; the price may be confirmed later by the owner.
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS lens_kind TEXT NOT NULL DEFAULT 'none' CHECK (lens_kind IN ('none', 'package', 'custom'));
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS lens_package_id INTEGER REFERENCES lens_packages(id) ON DELETE SET NULL;
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS lens_name TEXT NOT NULL DEFAULT '';
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS lens_power TEXT NOT NULL DEFAULT '' CHECK (lens_power IN ('', 'no_power', 'with_power'));
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS sph_right INTEGER CHECK (sph_right BETWEEN -2000 AND 1200);
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS sph_left INTEGER CHECK (sph_left BETWEEN -2000 AND 1200);
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS lens_price INTEGER NOT NULL DEFAULT 0 CHECK (lens_price >= 0);
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS lens_price_final BOOLEAN NOT NULL DEFAULT true;
-- Private file name under the prescription directory; never a public URL.
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS prescription_file TEXT NOT NULL DEFAULT '';
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS prescription_status TEXT NOT NULL DEFAULT '' CHECK (prescription_status IN ('', 'pending', 'accepted', 'needs_new'));
