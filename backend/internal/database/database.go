package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bongkey/backend/internal/catalog"
	"bongkey/backend/internal/config"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var DB *sql.DB

// Connect opens and verifies the shared PostgreSQL connection pool.
func Connect(databaseURL string) error {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return err
	}
	// Bounded pool sizes keep one instance from exhausting PostgreSQL connections; recycling avoids stale sockets.
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return err
	}
	DB = db
	return nil
}

func Ready() bool {
	return DB != nil
}

// MigrateAndSeed applies the current schema and idempotent migrations before local seed data.
func MigrateAndSeed(cfg config.Config) error {
	if DB == nil {
		return fmt.Errorf("database is not connected")
	}
	if _, err := DB.Exec(schemaSQL); err != nil {
		return err
	}
	for _, name := range []string{"002_guest_order_access.sql", "003_stock_quantity_nonnegative.sql", "004_email_otp.sql", "005_checkout_idempotency.sql", "006_order_fulfillment.sql", "007_delivery_settings.sql", "008_product_variants.sql", "009_delivery_zones.sql", "010_admin_login_limits.sql", "011_lens_packages.sql", "012_lens_orders.sql", "013_prescription_review.sql", "014_manual_payments.sql", "015_advance_policies.sql", "016_delivery_tracking.sql", "017_frame_line_price_final.sql", "018_notifications.sql", "019_order_risk_flags.sql", "020_customers.sql", "021_stock_history.sql", "022_audit_log.sql", "023_customer_passwords.sql"} {
		migration, err := os.ReadFile(filepath.Join(cfg.ProjectRoot, "backend", "migrations", name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := DB.Exec(string(migration)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	if err := seedAdmin(DB, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		return err
	}
	if err := seedCategories(); err != nil {
		return err
	}
	if err := seedProducts(); err != nil {
		return err
	}
	if err := seedLensPackages(); err != nil {
		return err
	}
	log.Println("database migrated and seeded")
	return nil
}

// seedAdmin bootstraps the first owner once without changing any existing credential.
func seedAdmin(db *sql.DB, email, password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize concurrent startup so two instances cannot bootstrap different owners.
	if _, err := tx.ExecContext(ctx, `LOCK TABLE admins IN EXCLUSIVE MODE`); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM admins`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return tx.Commit()
	}
	if strings.TrimSpace(email) == "" || len(password) < 12 || password == "admin12345" {
		return fmt.Errorf("first admin requires ADMIN_EMAIL and a non-template ADMIN_PASSWORD of at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admins (name, email, password_hash, role) VALUES ($1, $2, $3, 'owner')`, "Bongkey Owner", email, string(hash)); err != nil {
		return err
	}
	return tx.Commit()
}

func seedCategories() error {
	for _, name := range []string{"Blue-Cut Glasses", "Sunglasses", "Men's Frames", "Women's Frames", "Premium Collection", "Budget Picks", "Prescription-ready"} {
		_, err := DB.Exec(`INSERT INTO categories (name, slug) VALUES ($1, $2) ON CONFLICT (slug) DO NOTHING`, name, catalog.Slugify(name))
		if err != nil {
			return err
		}
	}
	return nil
}

// starterLens is one common lens type with a rough starting price for the owner to review.
type starterLens struct {
	Name, Description    string
	Price, MaxPower      int
	PrescriptionRequired bool
	CategorySlugs        []string
}

// eyeglassCategories are the seeded categories that hold optical frames.
var eyeglassCategories = []string{"blue-cut-glasses", "mens-frames", "womens-frames", "premium-collection", "budget-picks", "prescription-ready"}

// starterLenses are common lens types sold in Bangladesh. Prices are rough placeholders, not market quotes.
var starterLenses = []starterLens{
	{"Clear anti-glare", "Anti-glare coating\nUV protection\nEveryday wear", 1200, 600, false, eyeglassCategories},
	{"Blue-cut", "Filters screen blue light\nAnti-glare coating\nFor phone and computer use", 1800, 600, false, eyeglassCategories},
	{"Photochromic", "Clear indoors\nDarkens in sunlight\nUV protection", 2500, 600, false, eyeglassCategories},
	{"Photochromic + blue-cut", "Darkens in sunlight\nFilters screen blue light\nUV protection", 3500, 400, false, eyeglassCategories},
	{"Thin high-index 1.67", "Thinner and lighter\nFor strong powers\nAnti-glare coating", 4500, 1000, true, eyeglassCategories},
	{"Progressive (distance + reading)", "Far, middle and near in one lens\nNo visible line", 6500, 400, true, eyeglassCategories},
	{"Power tinted sunglass lens", "Your prescription in a dark sun lens\nUV protection", 2800, 400, true, []string{"sunglasses"}},
	{"Power polarized sunglass lens", "Your prescription, polarized\nCuts glare for driving and water", 3800, 400, true, []string{"sunglasses"}},
}

// seedLensPackages adds the starter lens types only when none exist, so the owner's later edits and renames are never overwritten.
func seedLensPackages() error {
	ctx := context.Background()
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM lens_packages`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for i, lens := range starterLenses {
		var id int
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO lens_packages (name, description, price, sort_order, max_power, prescription_required)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			lens.Name, lens.Description, lens.Price, (i+1)*10, lens.MaxPower, lens.PrescriptionRequired).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO lens_package_categories (package_id, category_id)
			SELECT $1, id FROM categories WHERE slug = ANY($2)`, id, pq.Array(lens.CategorySlugs)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type seedProduct struct {
	Name, Category, Gender, BrandTier, FrameType string
	Price, OldPrice, StockQuantity               int
	Material, Color, Size                        string
	LensWidth, BridgeWidth, TempleLength         string
	Featured                                     bool
}

// seedProducts inserts local catalog examples while products.stock_quantity remains the stock source.
func seedProducts() error {
	items := []seedProduct{
		{"Bongkey Classic Black Blue-Cut", "Blue-Cut Glasses", "unisex", "budget", "blue_cut", 1190, 1590, 18, "TR90", "Matte Black", "Medium", "52 mm", "18 mm", "140 mm", true},
		{"Bongkey Urban Clear Frame", "Blue-Cut Glasses", "unisex", "mid_range", "blue_cut", 1490, 1990, 14, "Acetate", "Crystal Clear", "Medium", "51 mm", "19 mm", "142 mm", true},
		{"Bongkey Premium Metal Round", "Premium Collection", "unisex", "premium", "premium_frame", 2690, 3290, 9, "Stainless Steel", "Champagne Gold", "Medium", "50 mm", "20 mm", "145 mm", true},
		{"Bongkey Daily Sunglass", "Sunglasses", "women", "mid_range", "sunglass", 1790, 2290, 20, "Acetate", "Tortoise Brown", "Medium", "54 mm", "17 mm", "142 mm", true},
		{"Bongkey Metal Pro Rectangle", "Men's Frames", "men", "premium", "prescription_ready", 2490, 2990, 7, "Stainless Steel", "Gunmetal", "Large", "55 mm", "19 mm", "145 mm", true},
		{"Bongkey Lite Round Everyday", "Budget Picks", "unisex", "budget", "prescription_ready", 990, 1290, 30, "Polycarbonate", "Crystal Grey", "Small", "49 mm", "20 mm", "138 mm", true},
		{"Bongkey Office Blue Slim", "Blue-Cut Glasses", "men", "mid_range", "blue_cut", 1590, 2090, 16, "TR90", "Navy Black", "Large", "54 mm", "18 mm", "145 mm", true},
		{"Bongkey Noor Gold Rim", "Women's Frames", "women", "premium", "premium_frame", 2290, 2790, 12, "Metal", "Soft Gold", "Small", "49 mm", "19 mm", "140 mm", true},
		{"Bongkey Dhaka Aviator", "Sunglasses", "men", "mid_range", "sunglass", 1890, 2390, 17, "Metal", "Black Gold", "Large", "58 mm", "14 mm", "145 mm", false},
		{"Bongkey Campus Clear Blue-Cut", "Blue-Cut Glasses", "unisex", "budget", "blue_cut", 1090, 1490, 25, "Polycarbonate", "Transparent Grey", "Medium", "51 mm", "18 mm", "140 mm", false},
		{"Bongkey Premium Acetate Square", "Premium Collection", "unisex", "premium", "premium_frame", 2990, 3590, 6, "Acetate", "Deep Havana", "Large", "53 mm", "18 mm", "145 mm", false},
		{"Bongkey Soft Pink Frame", "Women's Frames", "women", "budget", "prescription_ready", 1290, 1690, 19, "TR90", "Dusty Pink", "Small", "50 mm", "17 mm", "138 mm", false},
		{"Bongkey Executive Black", "Men's Frames", "men", "mid_range", "prescription_ready", 1690, 2190, 15, "TR90", "Charcoal Black", "Large", "55 mm", "18 mm", "145 mm", false},
		{"Bongkey Coastal Brown Sunglass", "Sunglasses", "unisex", "budget", "sunglass", 1390, 1790, 22, "Polycarbonate", "Coffee Brown", "Medium", "53 mm", "18 mm", "142 mm", false},
		{"Bongkey AirLite Blue-Cut", "Blue-Cut Glasses", "women", "premium", "blue_cut", 2590, 3190, 8, "Titanium Alloy", "Rose Gold", "Medium", "52 mm", "17 mm", "140 mm", false},
		{"Bongkey Student Black Frame", "Budget Picks", "unisex", "budget", "prescription_ready", 890, 1190, 35, "Polycarbonate", "Black", "Medium", "51 mm", "18 mm", "140 mm", false},
		{"Bongkey Chattogram Metal", "Men's Frames", "men", "premium", "premium_frame", 2790, 3390, 5, "Stainless Steel", "Silver", "Large", "54 mm", "18 mm", "145 mm", false},
		{"Bongkey Eid Edit Gold", "Premium Collection", "women", "premium", "premium_frame", 2890, 3490, 10, "Acetate Metal", "Pearl Gold", "Medium", "53 mm", "16 mm", "142 mm", false},
		{"Bongkey Minimal Grey", "Prescription-ready", "unisex", "mid_range", "prescription_ready", 1590, 2090, 18, "TR90", "Ash Grey", "Medium", "52 mm", "18 mm", "142 mm", false},
		{"Bongkey Sylhet Green Sunglass", "Sunglasses", "unisex", "mid_range", "sunglass", 1690, 2190, 13, "Metal", "Forest Green", "Medium", "51 mm", "20 mm", "142 mm", false},
	}
	for _, item := range items {
		slug := catalog.Slugify(item.Name)
		description := fmt.Sprintf("%s is a comfortable Bongkey frame for everyday use in Bangladesh.", item.Name)
		_, err := DB.Exec(`
			INSERT INTO products (
				category_id, name, slug, description, price, old_price, brand_tier, frame_type, gender,
				material, color, size, lens_width, bridge_width, temple_length, stock_quantity,
				image_url, try_on_image_url, is_featured, is_active
			)
			SELECT c.id, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, '', '', $16, true
			FROM categories c WHERE c.name = $17
			ON CONFLICT (slug) DO NOTHING
		`, item.Name, slug, description, item.Price, item.OldPrice, item.BrandTier, item.FrameType, item.Gender, item.Material, item.Color, item.Size, item.LensWidth, item.BridgeWidth, item.TempleLength, item.StockQuantity, item.Featured, item.Category)
		if err != nil {
			return err
		}
	}
	return nil
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS admins (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'owner',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS categories (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS products (
	id SERIAL PRIMARY KEY,
	category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	price INTEGER NOT NULL DEFAULT 0,
	old_price INTEGER NOT NULL DEFAULT 0,
	brand_tier TEXT NOT NULL DEFAULT 'budget',
	frame_type TEXT NOT NULL DEFAULT 'prescription_ready',
	gender TEXT NOT NULL DEFAULT 'unisex',
	material TEXT NOT NULL DEFAULT '',
	color TEXT NOT NULL DEFAULT '',
	size TEXT NOT NULL DEFAULT '',
	lens_width TEXT NOT NULL DEFAULT '',
	bridge_width TEXT NOT NULL DEFAULT '',
	temple_length TEXT NOT NULL DEFAULT '',
	stock_quantity INTEGER NOT NULL DEFAULT 0,
	image_url TEXT NOT NULL DEFAULT '',
	try_on_image_url TEXT NOT NULL DEFAULT '',
	is_featured BOOLEAN NOT NULL DEFAULT false,
	is_active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS product_images (
	id SERIAL PRIMARY KEY,
	product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
	image_url TEXT NOT NULL,
	alt_text TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS orders (
	id SERIAL PRIMARY KEY,
	order_number TEXT NOT NULL UNIQUE,
	customer_name TEXT NOT NULL,
	customer_phone TEXT NOT NULL,
	customer_email TEXT NOT NULL DEFAULT '',
	shipping_address TEXT NOT NULL,
	city TEXT NOT NULL,
	delivery_note TEXT NOT NULL DEFAULT '',
	prescription_file_url TEXT NOT NULL DEFAULT '',
	subtotal INTEGER NOT NULL,
	delivery_fee INTEGER NOT NULL,
	total INTEGER NOT NULL,
	payment_method TEXT NOT NULL DEFAULT 'cash_on_delivery',
	payment_status TEXT NOT NULL DEFAULT 'unpaid',
	order_status TEXT NOT NULL DEFAULT 'pending',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS order_items (
	id SERIAL PRIMARY KEY,
	order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
	product_id INTEGER REFERENCES products(id) ON DELETE SET NULL,
	product_name TEXT NOT NULL,
	sku TEXT NOT NULL DEFAULT '',
	lens_option TEXT NOT NULL DEFAULT 'Frame only',
	quantity INTEGER NOT NULL,
	unit_price INTEGER NOT NULL,
	total_price INTEGER NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS invoices (
	id SERIAL PRIMARY KEY,
	invoice_number TEXT NOT NULL UNIQUE,
	order_id INTEGER NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
	customer_name TEXT NOT NULL,
	customer_phone TEXT NOT NULL,
	customer_email TEXT NOT NULL DEFAULT '',
	billing_address TEXT NOT NULL,
	subtotal INTEGER NOT NULL,
	delivery_fee INTEGER NOT NULL,
	total INTEGER NOT NULL,
	issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
