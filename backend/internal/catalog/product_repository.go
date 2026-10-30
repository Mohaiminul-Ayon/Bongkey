package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"bongkey/backend/internal/models"
	"github.com/lib/pq"
)

// PostgresProductRepository persists catalog products in PostgreSQL.
type PostgresProductRepository struct{ DB *sql.DB }

const productSelectSQL = `
SELECT p.id, COALESCE(p.parent_product_id, 0), p.sku, COALESCE(p.category_id, 0), COALESCE(c.name, ''), p.name, p.slug, p.description,
	p.price, p.old_price, p.brand_tier, p.frame_type, p.gender, p.material, p.color, p.size,
	p.lens_width, p.bridge_width, p.temple_length, p.stock_quantity, p.image_url,
	p.try_on_image_url, p.is_featured, p.is_active
FROM products p
LEFT JOIN categories c ON c.id = p.category_id
`

// List reads products, applying the already-validated price range and sort order.
func (r PostgresProductRepository) List(ctx context.Context, featuredOnly, includeInactive bool, filter ListFilter) ([]models.Product, error) {
	if r.DB == nil {
		return nil, ErrUnavailable
	}
	// ORDER BY cannot take a $ parameter, so it is chosen from a fixed list, never from user text.
	orderBy := "p.id DESC"
	switch filter.Sort {
	case "price_asc":
		orderBy = "p.price ASC, p.id DESC"
	case "price_desc":
		orderBy = "p.price DESC, p.id DESC"
	}
	rows, err := r.DB.QueryContext(ctx, productSelectSQL+`
		WHERE ($1 OR (p.is_active = true AND p.parent_product_id IS NULL))
		AND ($2 = false OR p.is_featured = true)
		AND p.price >= $3
		AND ($4 = 0 OR p.price <= $4)
		ORDER BY `+orderBy, includeInactive, featuredOnly, filter.MinPrice, filter.MaxPrice)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]models.Product, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

// GetBySlug reads one published product.
func (r PostgresProductRepository) GetBySlug(ctx context.Context, slug string) (models.Product, error) {
	if r.DB == nil {
		return models.Product{}, ErrUnavailable
	}
	product, err := scanProduct(r.DB.QueryRowContext(ctx, productSelectSQL+` WHERE p.slug = $1 AND p.is_active = true AND (p.parent_product_id IS NULL OR EXISTS (SELECT 1 FROM products root WHERE root.id = p.parent_product_id AND root.is_active = true))`, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return models.Product{}, ErrNotFound
	}
	return product, err
}

// ListVariants reads one root and its independently stocked children.
func (r PostgresProductRepository) ListVariants(ctx context.Context, rootID int, includeInactive bool) ([]models.Product, error) {
	if r.DB == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.DB.QueryContext(ctx, productSelectSQL+` WHERE (p.id = $1 OR p.parent_product_id = $1) AND ($2 OR p.is_active = true) ORDER BY p.id`, rootID, includeInactive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	variants := make([]models.Product, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		variants = append(variants, product)
	}
	return variants, rows.Err()
}

// CreateVariant copies stable frame details while giving the new SKU its own price and stock.
func (r PostgresProductRepository) CreateVariant(ctx context.Context, rootID int, input VariantInput) (models.Product, error) {
	if r.DB == nil {
		return models.Product{}, ErrUnavailable
	}
	var id int
	err := r.DB.QueryRowContext(ctx, `
		INSERT INTO products (parent_product_id, sku, category_id, name, slug, description, price, old_price,
			brand_tier, frame_type, gender, material, color, size, lens_width, bridge_width, temple_length,
			stock_quantity, image_url, try_on_image_url, is_featured, is_active)
		SELECT p.id, $2, p.category_id, p.name, p.slug || '-' || LOWER($2), p.description, $3, 0,
			p.brand_tier, p.frame_type, p.gender, p.material, $4, $5, p.lens_width, p.bridge_width, p.temple_length,
			$6, p.image_url, p.try_on_image_url, false, p.is_active
		FROM products p WHERE p.id = $1 AND p.parent_product_id IS NULL
		RETURNING id
	`, rootID, input.SKU, input.Price, input.Color, input.Size, input.StockQuantity).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Product{}, ErrNotFound
	}
	if err != nil {
		return models.Product{}, variantWriteError(err)
	}
	return scanProduct(r.DB.QueryRowContext(ctx, productSelectSQL+` WHERE p.id = $1`, id))
}

// Save inserts or updates one product, then returns the stored owner view.
func (r PostgresProductRepository) Save(ctx context.Context, id int, input ProductInput) (models.Product, error) {
	if r.DB == nil {
		return models.Product{}, ErrUnavailable
	}
	if id == 0 {
		err := r.DB.QueryRowContext(ctx, `
			INSERT INTO products (
				category_id, name, slug, description, price, old_price, brand_tier, frame_type, gender,
				material, color, size, lens_width, bridge_width, temple_length, stock_quantity,
				image_url, try_on_image_url, is_featured, is_active, sku
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,COALESCE(NULLIF($21,''), 'BK-' || UPPER(SUBSTR(REPLACE(gen_random_uuid()::text, '-', ''), 1, 12))))
			RETURNING id
		`, input.CategoryID, input.Name, input.Slug, input.Description, input.Price, input.OldPrice, input.BrandTier, input.FrameType, input.Gender, input.Material, input.Color, input.Size, input.LensWidth, input.BridgeWidth, input.TempleLength, input.StockQuantity, input.ImageURL, input.TryOnImageURL, input.IsFeatured, input.IsActive, input.SKU).Scan(&id)
		if err != nil {
			return models.Product{}, variantWriteError(err)
		}
	} else {
		// The stock predicate prevents an old admin form from replacing a concurrent sale.
		result, err := r.DB.ExecContext(ctx, `
			UPDATE products SET category_id=$1, name=$2, slug=$3, description=$4, price=$5, old_price=$6,
				brand_tier=$7, frame_type=$8, gender=$9, material=$10, color=$11, size=$12,
				lens_width=$13, bridge_width=$14, temple_length=$15, stock_quantity=$16,
				image_url=$17, try_on_image_url=$18, is_featured=$19, is_active=$20, sku=COALESCE(NULLIF($23,''),sku), updated_at=NOW()
			WHERE id=$21 AND stock_quantity=$22
		`, input.CategoryID, input.Name, input.Slug, input.Description, input.Price, input.OldPrice, input.BrandTier, input.FrameType, input.Gender, input.Material, input.Color, input.Size, input.LensWidth, input.BridgeWidth, input.TempleLength, input.StockQuantity, input.ImageURL, input.TryOnImageURL, input.IsFeatured, input.IsActive, id, *input.ExpectedStockQuantity, input.SKU)
		if err != nil {
			return models.Product{}, variantWriteError(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return models.Product{}, err
		}
		if affected == 0 {
			return models.Product{}, ErrStaleStock
		}
	}
	return scanProduct(r.DB.QueryRowContext(ctx, productSelectSQL+` WHERE p.id = $1`, id))
}

// Delete removes a product only when the database allows it.
func (r PostgresProductRepository) Delete(ctx context.Context, id int) error {
	if r.DB == nil {
		return ErrUnavailable
	}
	_, err := r.DB.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, id)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23503" {
		return ErrHasVariants
	}
	return err
}

// SetActive changes one product's publication state.
func (r PostgresProductRepository) SetActive(ctx context.Context, id int, active bool) error {
	if r.DB == nil {
		return ErrUnavailable
	}
	_, err := r.DB.ExecContext(ctx, `UPDATE products SET is_active=$1, updated_at=NOW() WHERE id=$2`, active, id)
	return err
}

type productScanner interface{ Scan(...any) error }

// scanProduct keeps the read column mapping next to its SQL definition.
func scanProduct(row productScanner) (models.Product, error) {
	var product models.Product
	err := row.Scan(&product.ID, &product.ParentProductID, &product.SKU, &product.CategoryID, &product.CategoryName, &product.Name, &product.Slug, &product.Description, &product.Price, &product.OldPrice, &product.BrandTier, &product.FrameType, &product.Gender, &product.Material, &product.Color, &product.Size, &product.LensWidth, &product.BridgeWidth, &product.TempleLength, &product.StockQuantity, &product.ImageURL, &product.TryOnImageURL, &product.IsFeatured, &product.IsActive)
	return product, err
}

// variantWriteError turns uniqueness failures into a stable catalog error.
func variantWriteError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23505" {
		return ErrDuplicateVariant
	}
	return fmt.Errorf("save product variant: %w", err)
}
