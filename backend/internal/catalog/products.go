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
)

// ProductInput is the set of product fields the owner may edit.
type ProductInput struct {
	CategoryID, Price, OldPrice, StockQuantity int
	ExpectedStockQuantity                      *int
	Name, Slug, Description                    string
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
}

// ProductService owns editable product rules and delegates persistence.
type ProductService struct{ Repository ProductRepository }

// List returns visible products or the full owner view.
func (s ProductService) List(ctx context.Context, featuredOnly, includeInactive bool) ([]models.Product, error) {
	return s.Repository.List(ctx, featuredOnly, includeInactive)
}

// GetBySlug returns one published product.
func (s ProductService) GetBySlug(ctx context.Context, slug string) (models.Product, error) {
	return s.Repository.GetBySlug(ctx, slug)
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
