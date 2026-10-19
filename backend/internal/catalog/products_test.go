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
