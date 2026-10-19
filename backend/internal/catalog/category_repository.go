package catalog

import (
	"context"
	"database/sql"

	"bongkey/backend/internal/models"
)

// PostgresCategoryRepository persists categories in PostgreSQL.
type PostgresCategoryRepository struct{ DB *sql.DB }

// List reads categories in their existing display order.
func (r PostgresCategoryRepository) List(ctx context.Context) ([]models.Category, error) {
	if r.DB == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT id, name, slug, description FROM categories ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	categories := make([]models.Category, 0)
	for rows.Next() {
		var category models.Category
		if err := rows.Scan(&category.ID, &category.Name, &category.Slug, &category.Description); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

// Save inserts or updates only category fields that the owner can edit.
func (r PostgresCategoryRepository) Save(ctx context.Context, id int, category models.Category) error {
	if r.DB == nil {
		return ErrUnavailable
	}
	if id == 0 {
		_, err := r.DB.ExecContext(ctx, `INSERT INTO categories (name, slug, description) VALUES ($1,$2,$3)`, category.Name, category.Slug, category.Description)
		return err
	}
	_, err := r.DB.ExecContext(ctx, `UPDATE categories SET name=$1, slug=$2, description=$3, updated_at=NOW() WHERE id=$4`, category.Name, category.Slug, category.Description, id)
	return err
}

// Delete removes one category by ID.
func (r PostgresCategoryRepository) Delete(ctx context.Context, id int) error {
	if r.DB == nil {
		return ErrUnavailable
	}
	_, err := r.DB.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, id)
	return err
}
