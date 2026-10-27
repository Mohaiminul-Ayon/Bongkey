package catalog

import (
	"context"
	"errors"
	"strings"

	"bongkey/backend/internal/models"
)

// ErrCategoryNameRequired rejects empty category names before persistence.
var ErrCategoryNameRequired = errors.New("category name required")

// CategoryRepository stores categories without HTTP concerns.
type CategoryRepository interface {
	List(context.Context) ([]models.Category, error)
	Save(context.Context, int, models.Category) error
	Delete(context.Context, int) error
}

// CategoryService owns category input rules.
type CategoryService struct{ Repository CategoryRepository }

// List returns categories for both the storefront and owner.
func (s CategoryService) List(ctx context.Context) ([]models.Category, error) {
	return s.Repository.List(ctx)
}

// Save validates a category and derives its default slug.
func (s CategoryService) Save(ctx context.Context, id int, category models.Category) error {
	category.Name = strings.TrimSpace(category.Name)
	if category.Name == "" {
		return ErrCategoryNameRequired
	}
	if strings.TrimSpace(category.Slug) == "" {
		category.Slug = Slugify(category.Name)
	}
	return s.Repository.Save(ctx, id, category)
}

// Delete removes a category when database references permit it.
func (s CategoryService) Delete(ctx context.Context, id int) error {
	return s.Repository.Delete(ctx, id)
}
