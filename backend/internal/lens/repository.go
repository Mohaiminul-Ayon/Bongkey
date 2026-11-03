package lens

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

// PostgresRepository persists lens packages in PostgreSQL.
type PostgresRepository struct{ DB *sql.DB }

// ListAll reads every package with its category links in display order.
func (r PostgresRepository) ListAll(ctx context.Context) ([]Package, error) {
	if r.DB == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT p.id, p.name, p.description, p.price, p.sort_order, p.active,
			COALESCE(array_agg(c.category_id ORDER BY c.category_id) FILTER (WHERE c.category_id IS NOT NULL), '{}')
		FROM lens_packages p LEFT JOIN lens_package_categories c ON c.package_id = p.id
		GROUP BY p.id ORDER BY p.sort_order, p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	packages := make([]Package, 0)
	for rows.Next() {
		var pkg Package
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Description, &pkg.Price, &pkg.SortOrder, &pkg.Active, pq.Array(&pkg.CategoryIDs)); err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	return packages, rows.Err()
}

// ListForProduct reads enabled packages linked to the category of one active frame.
func (r PostgresRepository) ListForProduct(ctx context.Context, slug string) ([]Package, error) {
	if r.DB == nil {
		return nil, ErrUnavailable
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT p.id, p.name, p.description, p.price, p.sort_order
		FROM lens_packages p
		JOIN lens_package_categories c ON c.package_id = p.id
		JOIN products pr ON pr.category_id = c.category_id
		WHERE pr.slug = $1 AND pr.is_active AND p.active
		ORDER BY p.sort_order, p.id`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	packages := make([]Package, 0)
	for rows.Next() {
		pkg := Package{Active: true}
		if err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Description, &pkg.Price, &pkg.SortOrder); err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	return packages, rows.Err()
}

// Save writes the package and replaces its category links atomically, so a package never fits a half-saved set.
func (r PostgresRepository) Save(ctx context.Context, id int, pkg Package) error {
	if r.DB == nil {
		return ErrUnavailable
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if id == 0 {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO lens_packages (name, description, price, sort_order, active) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			pkg.Name, pkg.Description, pkg.Price, pkg.SortOrder, pkg.Active).Scan(&id)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `
			UPDATE lens_packages SET name = $1, description = $2, price = $3, sort_order = $4, active = $5, updated_at = NOW() WHERE id = $6`,
			pkg.Name, pkg.Description, pkg.Price, pkg.SortOrder, pkg.Active, id)
		if err == nil {
			if affected, _ := result.RowsAffected(); affected == 0 {
				return ErrNotFound
			}
		}
	}
	if err != nil {
		return storageError(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM lens_package_categories WHERE package_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO lens_package_categories (package_id, category_id) SELECT $1, unnest($2::int[])`, id, pq.Array(pkg.CategoryIDs)); err != nil {
		return storageError(err)
	}
	return tx.Commit()
}

// storageError turns constraint failures into package errors the handler can explain.
func storageError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "23505":
			return ErrDuplicateName
		case "23503", "23514":
			return ErrInvalidPackage
		}
	}
	return err
}
