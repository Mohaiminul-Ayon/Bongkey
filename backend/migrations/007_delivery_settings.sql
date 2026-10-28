CREATE TABLE IF NOT EXISTS store_settings (
	id SMALLINT PRIMARY KEY CHECK (id = 1),
	 delivery_fee INTEGER NOT NULL CHECK (delivery_fee >= 0)
);

INSERT INTO store_settings (id, delivery_fee)
VALUES (1, 80)
ON CONFLICT (id) DO NOTHING;
