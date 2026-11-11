// Package lens owns the fixed-price lens packages sold with frames.
package lens

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	// ErrInvalidPackage means a package field or category link cannot be stored.
	ErrInvalidPackage = errors.New("invalid lens package")
	// ErrDuplicateName means another package already uses the name shoppers see.
	ErrDuplicateName = errors.New("lens package name already used")
	// ErrNotFound means the package to edit does not exist.
	ErrNotFound = errors.New("lens package not found")
	// ErrUnavailable means lens storage is not connected.
	ErrUnavailable = errors.New("lens storage unavailable")
)

// Package is one lens type with a fixed price, linked to the frame categories it fits.
type Package struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	// Description holds the feature lines shown on the shop's lens card, one per line.
	Description string  `json:"description"`
	Price       int     `json:"price"`
	SortOrder   int     `json:"sort_order"`
	Active      bool    `json:"active"`
	CategoryIDs []int64 `json:"category_ids"`
	// MaxPower is the highest SPH, in hundredths (600 = ±6.00), that the fixed price covers.
	MaxPower int `json:"max_power"`
	// PrescriptionRequired removes the "no power" choice, e.g. for power sunglass lenses.
	PrescriptionRequired bool `json:"prescription_required"`
}

// Repository stores packages; Save replaces the category links in the same transaction.
type Repository interface {
	ListAll(context.Context) ([]Package, error)
	ListForProduct(ctx context.Context, slug string) ([]Package, error)
	Save(ctx context.Context, id int, pkg Package) error
}

// Service owns lens package input rules.
type Service struct{ Repository Repository }

// ListAll returns every package, including disabled ones, for the owner.
func (s Service) ListAll(ctx context.Context) ([]Package, error) { return s.Repository.ListAll(ctx) }

// ForProduct returns the enabled packages that fit one active frame.
func (s Service) ForProduct(ctx context.Context, slug string) ([]Package, error) {
	return s.Repository.ListForProduct(ctx, slug)
}

// featureLines trims each feature line and drops blank ones, so the shop never shows empty bullets.
func featureLines(text string) string {
	lines := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// Save validates a package; id 0 creates a new one.
func (s Service) Save(ctx context.Context, id int, pkg Package) error {
	pkg.Name = strings.TrimSpace(pkg.Name)
	pkg.Description = featureLines(pkg.Description)
	if pkg.Name == "" || utf8.RuneCountInString(pkg.Name) > 80 || utf8.RuneCountInString(pkg.Description) > 300 ||
		pkg.Price <= 0 || pkg.Price > 200000 || pkg.SortOrder < 0 || pkg.SortOrder > 999 || len(pkg.CategoryIDs) > 50 ||
		pkg.MaxPower < 25 || pkg.MaxPower > 2000 || pkg.MaxPower%25 != 0 {
		return ErrInvalidPackage
	}
	seen := make(map[int64]bool, len(pkg.CategoryIDs))
	unique := make([]int64, 0, len(pkg.CategoryIDs))
	for _, categoryID := range pkg.CategoryIDs {
		if categoryID <= 0 {
			return ErrInvalidPackage
		}
		if !seen[categoryID] {
			seen[categoryID] = true
			unique = append(unique, categoryID)
		}
	}
	pkg.CategoryIDs = unique
	return s.Repository.Save(ctx, id, pkg)
}
