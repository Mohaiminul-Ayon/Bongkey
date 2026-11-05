CREATE TABLE IF NOT EXISTS lens_packages (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	price INTEGER NOT NULL CHECK (price > 0),
	sort_order INTEGER NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A package fits every frame in the linked categories.
CREATE TABLE IF NOT EXISTS lens_package_categories (
	package_id INTEGER NOT NULL REFERENCES lens_packages(id) ON DELETE CASCADE,
	category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
	PRIMARY KEY (package_id, category_id)
);
