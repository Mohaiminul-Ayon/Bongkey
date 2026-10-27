package catalog

import (
	"context"
	"errors"
	"testing"
)

// TestProductServiceRejectsInvalidEdits checks business input before storage is touched.
func TestProductServiceRejectsInvalidEdits(t *testing.T) {
	stock := 2
	base := ProductInput{CategoryID: 1, Name: "Frame", Price: 100, StockQuantity: 2, ExpectedStockQuantity: &stock}
	for _, test := range []struct {
		name  string
		id    int
		input ProductInput
		want  error
	}{
		{"missing category", 0, ProductInput{Name: "Frame", Price: 100}, ErrCategoryRequired},
		{"missing name", 0, ProductInput{CategoryID: 1, Price: 100}, ErrNamePriceRequired},
		{"negative stock", 0, ProductInput{CategoryID: 1, Name: "Frame", Price: 100, StockQuantity: -1}, ErrNegativeStock},
		{"stale edit guard missing", 7, ProductInput{CategoryID: 1, Name: "Frame", Price: 100}, ErrExpectedStockMissing},
		{"valid edit reaches storage", 7, base, ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := ProductService{Repository: PostgresProductRepository{}}
			_, err := service.Save(context.Background(), test.id, test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("Save error = %v, want %v", err, test.want)
			}
		})
	}
}

// TestCreateVariantRejectsInvalidOptions keeps malformed SKUs and impossible stock out of persistence.
func TestCreateVariantRejectsInvalidOptions(t *testing.T) {
	service := ProductService{Repository: PostgresProductRepository{}}
	for _, input := range []VariantInput{
		{SKU: "", Color: "Black", Size: "M", Price: 100, StockQuantity: 1},
		{SKU: "bad space", Color: "Black", Size: "M", Price: 100, StockQuantity: 1},
		{SKU: "BK-BLK-M", Color: "", Size: "M", Price: 100, StockQuantity: 1},
		{SKU: "BK-BLK-M", Color: "Black", Size: "M", Price: 0, StockQuantity: 1},
		{SKU: "BK-BLK-M", Color: "Black", Size: "M", Price: 100, StockQuantity: -1},
		{SKU: "BK-BLK-M", Color: "Black", Size: "M", Price: 1 << 31, StockQuantity: 1},
	} {
		if _, err := service.CreateVariant(context.Background(), 1, input); !errors.Is(err, ErrInvalidVariant) {
			t.Fatalf("CreateVariant(%+v) error = %v", input, err)
		}
	}
	if _, err := service.CreateVariant(context.Background(), 1, VariantInput{SKU: "BK-BLK-M", Color: "Black", Size: "M", Price: 100, StockQuantity: 1}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("valid variant error = %v; want repository call", err)
	}
}
