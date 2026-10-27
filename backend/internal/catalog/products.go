package catalog

import (
	"context"
	"errors"
	"strings"

	"bongkey/backend/internal/models"
)

var (
	ErrUnavailable          = errors.New("catalog storage unavailable")
	ErrNotFound             = errors.New("product not found")
	ErrStaleStock           = errors.New("product stock changed")
	ErrCategoryRequired     = errors.New("category required")
	ErrNamePriceRequired    = errors.New("name and price required")
	ErrNegativeStock        = errors.New("negative stock")
	ErrExpectedStockMissing = errors.New("expected stock required")
	ErrInvalidVariant       = errors.New("invalid variant")
	ErrDuplicateVariant     = errors.New("duplicate variant")
	ErrHasVariants          = errors.New("product has variants")
)

// ProductInput is the set of product fields the owner may edit.
type ProductInput struct {
	CategoryID, Price, OldPrice, StockQuantity int
	ExpectedStockQuantity                      *int
	Name, Slug, Description                    string
	SKU                                        string
	BrandTier, FrameType, Gender               string
	Material, Color, Size                      string
	LensWidth, BridgeWidth, TempleLength       string
	ImageURL, TryOnImageURL                    string
	IsFeatured, IsActive                       bool
}

// ProductRepository stores catalog products without HTTP concerns.
type ProductRepository interface {
	List(context.Context, bool, bool) ([]models.Product, error)
	GetBySlug(context.Context, string) (models.Product, error)
	Save(context.Context, int, ProductInput) (models.Product, error)
	Delete(context.Context, int) error
	SetActive(context.Context, int, bool) error
	CreateVariant(context.Context, int, VariantInput) (models.Product, error)
	ListVariants(context.Context, int, bool) ([]models.Product, error)
}

// VariantInput describes one sellable size and color with independent stock.
type VariantInput struct {
	SKU, Color, Size     string
	Price, StockQuantity int
}

// ProductService owns editable product rules and delegates persistence.
type ProductService struct{ Repository ProductRepository }

// List returns visible products or the full owner view.
func (s ProductService) List(ctx context.Context, featuredOnly, includeInactive bool) ([]models.Product, error) {
	return s.Repository.List(ctx, featuredOnly, includeInactive)
}

// GetBySlug returns one published product.
func (s ProductService) GetBySlug(ctx context.Context, slug string) (models.Product, error) {
	product, err := s.Repository.GetBySlug(ctx, slug)
	if err != nil {
		return product, err
	}
	rootID := product.ID
	if product.ParentProductID != 0 {
		rootID = product.ParentProductID
	}
	product.Variants, err = s.Repository.ListVariants(ctx, rootID, false)
	return product, err
}

// ListVariants returns every variant for owner editing or active variants for shoppers.
func (s ProductService) ListVariants(ctx context.Context, rootID int, includeInactive bool) ([]models.Product, error) {
	return s.Repository.ListVariants(ctx, rootID, includeInactive)
}

// CreateVariant validates a distinct sellable SKU before copying shared frame details.
func (s ProductService) CreateVariant(ctx context.Context, rootID int, input VariantInput) (models.Product, error) {
	input.SKU = strings.TrimSpace(input.SKU)
	input.Color = strings.TrimSpace(input.Color)
	input.Size = strings.TrimSpace(input.Size)
	if rootID <= 0 || !validSKU(input.SKU) || input.Color == "" || input.Size == "" || input.Price <= 0 || input.Price > 1<<31-1 || input.StockQuantity < 0 || input.StockQuantity > 1<<31-1 {
		return models.Product{}, ErrInvalidVariant
	}
	return s.Repository.CreateVariant(ctx, rootID, input)
}

// Save validates product rules and protects updates against stale stock edits.
func (s ProductService) Save(ctx context.Context, id int, input ProductInput) (models.Product, error) {
	if input.CategoryID <= 0 {
		return models.Product{}, ErrCategoryRequired
	}
	if strings.TrimSpace(input.Name) == "" || input.Price <= 0 {
		return models.Product{}, ErrNamePriceRequired
	}
	if input.StockQuantity < 0 {
		return models.Product{}, ErrNegativeStock
	}
	if input.SKU != "" {
		input.SKU = strings.TrimSpace(input.SKU)
		if !validSKU(input.SKU) {
			return models.Product{}, ErrInvalidVariant
		}
	}
	if id != 0 && (input.ExpectedStockQuantity == nil || *input.ExpectedStockQuantity < 0) {
		return models.Product{}, ErrExpectedStockMissing
	}
	if input.Slug == "" {
		input.Slug = slugify(input.Name)
	}
	if input.BrandTier == "" {
		input.BrandTier = "budget"
	}
	if input.FrameType == "" {
		input.FrameType = "prescription_ready"
	}
	if input.Gender == "" {
		input.Gender = "unisex"
	}
	return s.Repository.Save(ctx, id, input)
}

// validSKU keeps owner-defined identifiers safe for a predictable URL suffix.
func validSKU(sku string) bool {
	if len(sku) < 2 || len(sku) > 64 {
		return false
	}
	for _, char := range sku {
		if (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') && (char < '0' || char > '9') && (char != '-' && char != '_') {
			return false
		}
	}
	return true
}

// Delete removes an unreferenced product; the database preserves referenced order history.
func (s ProductService) Delete(ctx context.Context, id int) error {
	return s.Repository.Delete(ctx, id)
}

// SetActive changes storefront visibility without changing accepted orders.
func (s ProductService) SetActive(ctx context.Context, id int, active bool) error {
	return s.Repository.SetActive(ctx, id, active)
}

// slugify derives the default URL slug from a product name.
func slugify(value string) string {
	slug := strings.ToLower(value)
	replacer := strings.NewReplacer("'", "", " ", "-", "/", "-", "’", "", "(", "", ")", "")
	slug = replacer.Replace(slug)
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return strings.Trim(slug, "-")
}
