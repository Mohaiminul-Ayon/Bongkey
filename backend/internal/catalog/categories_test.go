package catalog

import (
	"context"
	"errors"
	"testing"

	"bongkey/backend/internal/models"
)

type categoryStoreStub struct {
	saved     models.Category
	saveCalls int
}

// List returns no categories in this focused service test.
func (s *categoryStoreStub) List(context.Context) ([]models.Category, error) { return nil, nil }

// Save records the normalized input passed to persistence.
func (s *categoryStoreStub) Save(_ context.Context, _ int, category models.Category) error {
	s.saved = category
	s.saveCalls++
	return nil
}

// Delete is unused by the category validation test.
func (s *categoryStoreStub) Delete(context.Context, int) error { return nil }

// TestCategoryServiceSave checks validation and default slug generation.
func TestCategoryServiceSave(t *testing.T) {
	store := &categoryStoreStub{}
	service := CategoryService{Repository: store}
	if err := service.Save(context.Background(), 0, models.Category{Name: "  "}); !errors.Is(err, ErrCategoryNameRequired) || store.saveCalls != 0 {
		t.Fatalf("empty name should fail before persistence: err=%v calls=%d", err, store.saveCalls)
	}
	if err := service.Save(context.Background(), 0, models.Category{Name: "  New Frames  "}); err != nil {
		t.Fatal(err)
	}
	if store.saved.Name != "New Frames" || store.saved.Slug != "new-frames" {
		t.Fatalf("unexpected normalized category: %+v", store.saved)
	}
}
