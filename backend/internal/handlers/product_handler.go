package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"bongkey/backend/internal/database"
	"bongkey/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type productRequest struct {
	CategoryID    int    `json:"category_id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Description   string `json:"description"`
	Price         int    `json:"price"`
	OldPrice      int    `json:"old_price"`
	BrandTier     string `json:"brand_tier"`
	FrameType     string `json:"frame_type"`
	Gender        string `json:"gender"`
	Material      string `json:"material"`
	Color         string `json:"color"`
	Size          string `json:"size"`
	LensWidth     string `json:"lens_width"`
	BridgeWidth   string `json:"bridge_width"`
	TempleLength  string `json:"temple_length"`
	StockQuantity int    `json:"stock_quantity"`
	ImageURL      string `json:"image_url"`
	TryOnImageURL string `json:"try_on_image_url"`
	IsFeatured    bool   `json:"is_featured"`
	IsActive      bool   `json:"is_active"`
}

func ListProducts(c *gin.Context) {
	listProducts(c, false, false)
}

func ListFeaturedProducts(c *gin.Context) {
	listProducts(c, true, false)
}

func AdminListProducts(c *gin.Context) {
	listProducts(c, false, true)
}

func listProducts(c *gin.Context, featuredOnly bool, includeInactive bool) {
	if !database.Ready() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not connected"})
		return
	}
	where := "WHERE ($1 OR p.is_active = true) AND ($2 = false OR p.is_featured = true)"
	rows, err := database.DB.Query(productSelectSQL+" "+where+" ORDER BY p.id DESC", includeInactive, featuredOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load products"})
		return
	}
	defer rows.Close()
	products, err := scanProducts(rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read products"})
		return
	}
	c.JSON(http.StatusOK, products)
}

func GetProduct(c *gin.Context) {
	if !database.Ready() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not connected"})
		return
	}
	row := database.DB.QueryRow(productSelectSQL+" WHERE p.slug = $1 AND p.is_active = true", c.Param("slug"))
	product, err := scanProduct(row)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load product"})
		return
	}
	c.JSON(http.StatusOK, product)
}

func CreateProduct(c *gin.Context) {
	saveProduct(c, 0)
}

func UpdateProduct(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	saveProduct(c, id)
}

func saveProduct(c *gin.Context, id int) {
	if !database.Ready() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not connected"})
		return
	}
	var req productRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product data"})
		return
	}
	if gin.Mode() == gin.DebugMode {
		log.Printf("admin product save id=%d category_id=%d name=%q slug=%q price=%d old_price=%d stock_quantity=%d image_url=%q try_on_image_url=%q", id, req.CategoryID, req.Name, req.Slug, req.Price, req.OldPrice, req.StockQuantity, req.ImageURL, req.TryOnImageURL)
	}
	if req.CategoryID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please select a category."})
		return
	}
	if req.Name == "" || req.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "product name and price are required"})
		return
	}
	if req.Slug == "" {
		req.Slug = slugify(req.Name)
	}
	if req.BrandTier == "" {
		req.BrandTier = "budget"
	}
	if req.FrameType == "" {
		req.FrameType = "prescription_ready"
	}
	if req.Gender == "" {
		req.Gender = "unisex"
	}

	if id == 0 {
		err := database.DB.QueryRow(`
			INSERT INTO products (
				category_id, name, slug, description, price, old_price, brand_tier, frame_type, gender,
				material, color, size, lens_width, bridge_width, temple_length, stock_quantity,
				image_url, try_on_image_url, is_featured, is_active
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
			RETURNING id
		`, req.CategoryID, req.Name, req.Slug, req.Description, req.Price, req.OldPrice, req.BrandTier, req.FrameType, req.Gender, req.Material, req.Color, req.Size, req.LensWidth, req.BridgeWidth, req.TempleLength, req.StockQuantity, req.ImageURL, req.TryOnImageURL, req.IsFeatured, req.IsActive).Scan(&id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not create product: %v", err)})
			return
		}
	} else {
		_, err := database.DB.Exec(`
			UPDATE products SET category_id=$1, name=$2, slug=$3, description=$4, price=$5, old_price=$6,
				brand_tier=$7, frame_type=$8, gender=$9, material=$10, color=$11, size=$12,
				lens_width=$13, bridge_width=$14, temple_length=$15, stock_quantity=$16,
				image_url=$17, try_on_image_url=$18, is_featured=$19, is_active=$20, updated_at=NOW()
			WHERE id=$21
		`, req.CategoryID, req.Name, req.Slug, req.Description, req.Price, req.OldPrice, req.BrandTier, req.FrameType, req.Gender, req.Material, req.Color, req.Size, req.LensWidth, req.BridgeWidth, req.TempleLength, req.StockQuantity, req.ImageURL, req.TryOnImageURL, req.IsFeatured, req.IsActive, id)
		if err != nil {
			log.Printf("could not update product id=%d: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not update product: %v", err)})
			return
		}
	}

	_, _ = database.DB.Exec(`
		INSERT INTO stock_items (product_id, sku, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity, updated_at = NOW()
	`, id, strings.ToUpper(strings.ReplaceAll(req.Slug, "-", "")), req.StockQuantity)

	row := database.DB.QueryRow(productSelectSQL+" WHERE p.id = $1", id)
	product, err := scanProduct(row)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "product saved but could not reload"})
		return
	}
	c.JSON(http.StatusOK, product)
}

func DeleteProduct(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, err := database.DB.Exec(`DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete product"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func PatchProductStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_active is required"})
		return
	}
	_, err := database.DB.Exec(`UPDATE products SET is_active=$1, updated_at=NOW() WHERE id=$2`, req.IsActive, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": true})
}

const productSelectSQL = `
SELECT p.id, COALESCE(p.category_id, 0), COALESCE(c.name, ''), p.name, p.slug, p.description,
	p.price, p.old_price, p.brand_tier, p.frame_type, p.gender, p.material, p.color, p.size,
	p.lens_width, p.bridge_width, p.temple_length, p.stock_quantity, p.image_url,
	p.try_on_image_url, p.is_featured, p.is_active
FROM products p
LEFT JOIN categories c ON c.id = p.category_id
`

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanProduct(row scanner) (models.Product, error) {
	var p models.Product
	err := row.Scan(&p.ID, &p.CategoryID, &p.CategoryName, &p.Name, &p.Slug, &p.Description, &p.Price, &p.OldPrice, &p.BrandTier, &p.FrameType, &p.Gender, &p.Material, &p.Color, &p.Size, &p.LensWidth, &p.BridgeWidth, &p.TempleLength, &p.StockQuantity, &p.ImageURL, &p.TryOnImageURL, &p.IsFeatured, &p.IsActive)
	return p, err
}

func scanProducts(rows *sql.Rows) ([]models.Product, error) {
	var products []models.Product
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func slugify(value string) string {
	slug := strings.ToLower(value)
	replacer := strings.NewReplacer("'", "", " ", "-", "/", "-", "’", "", "(", "", ")", "")
	slug = replacer.Replace(slug)
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return strings.Trim(slug, "-")
}
