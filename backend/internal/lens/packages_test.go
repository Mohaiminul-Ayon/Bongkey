package lens

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type packageStoreStub struct {
	saved     Package
	saveCalls int
}

// ListAll is unused by the validation test.
func (s *packageStoreStub) ListAll(context.Context) ([]Package, error) { return nil, nil }

// ListForProduct is unused by the validation test.
func (s *packageStoreStub) ListForProduct(context.Context, string) ([]Package, error) {
	return nil, nil
}

// Save records the normalized package passed to storage.
func (s *packageStoreStub) Save(_ context.Context, _ int, pkg Package) error {
	s.saved = pkg
	s.saveCalls++
	return nil
}

// TestServiceSaveRejectsInvalidPackages stops bad input before storage.
func TestServiceSaveRejectsInvalidPackages(t *testing.T) {
	valid := Package{Name: "Blue-cut", Price: 2500, CategoryIDs: []int64{1}}
	for name, change := range map[string]func(*Package){
		"blank name":       func(p *Package) { p.Name = "  " },
		"long name":        func(p *Package) { p.Name = strings.Repeat("a", 81) },
		"zero price":       func(p *Package) { p.Price = 0 },
		"negative price":   func(p *Package) { p.Price = -10 },
		"huge price":       func(p *Package) { p.Price = 200001 },
		"negative order":   func(p *Package) { p.SortOrder = -1 },
		"bad category":     func(p *Package) { p.CategoryIDs = []int64{0} },
		"long description": func(p *Package) { p.Description = strings.Repeat("a", 301) },
	} {
		store := &packageStoreStub{}
		pkg := valid
		change(&pkg)
		if err := (Service{Repository: store}).Save(context.Background(), 0, pkg); !errors.Is(err, ErrInvalidPackage) || store.saveCalls != 0 {
			t.Fatalf("%s: err=%v saveCalls=%d", name, err, store.saveCalls)
		}
	}
}

// TestServiceSaveNormalizesInput trims text and drops repeated categories.
func TestServiceSaveNormalizesInput(t *testing.T) {
	store := &packageStoreStub{}
	err := (Service{Repository: store}).Save(context.Background(), 0, Package{Name: "  Photochromic ", Price: 3500, CategoryIDs: []int64{2, 1, 2}})
	if err != nil || store.saved.Name != "Photochromic" || len(store.saved.CategoryIDs) != 2 {
		t.Fatalf("saved=%+v err=%v", store.saved, err)
	}
}
