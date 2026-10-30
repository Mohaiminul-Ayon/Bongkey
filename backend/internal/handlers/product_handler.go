package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"bongkey/backend/internal/catalog"

	"github.com/gin-gonic/gin"
)

type productRequest struct {
	CategoryID            int    `json:"category_id"`
	Name                  string `json:"name"`
	SKU                   string `json:"sku"`
	Slug                  string `json:"slug"`
	Description           string `json:"description"`
	Price                 int    `json:"price"`
	OldPrice              int    `json:"old_price"`
	BrandTier             string `json:"brand_tier"`
	FrameType             string `json:"frame_type"`
	Gender                string `json:"gender"`
	Material              string `json:"material"`
	Color                 string `json:"color"`
	Size                  string `json:"size"`
	LensWidth             string `json:"lens_width"`
	BridgeWidth           string `json:"bridge_width"`
	TempleLength          string `json:"temple_length"`
	StockQuantity         int    `json:"stock_quantity"`
	ExpectedStockQuantity *int   `json:"expected_stock_quantity"`
	ImageURL              string `json:"image_url"`
	TryOnImageURL         string `json:"try_on_image_url"`
	IsFeatured            bool   `json:"is_featured"`
	IsActive              bool   `json:"is_active"`
}

// ProductHandler maps product HTTP requests to catalog decisions.
type ProductHandler struct{ Service catalog.ProductService }

type variantRequest struct {
	SKU           string `json:"sku"`
	Color         string `json:"color"`
	Size          string `json:"size"`
	Price         int    `json:"price"`
	StockQuantity int    `json:"stock_quantity"`
}

// ListVariants returns owner-visible sizes and colors for one product group.
func (h ProductHandler) ListVariants(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	variants, err := h.Service.ListVariants(c.Request.Context(), id, true)
	if err != nil {
		productError(c, err, "could not load variants")
		return
	}
	c.JSON(http.StatusOK, variants)
}

// CreateVariant accepts one independent SKU under an existing root product.
func (h ProductHandler) CreateVariant(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req variantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid variant data"})
		return
	}
	variant, err := h.Service.CreateVariant(c.Request.Context(), id, catalog.VariantInput{SKU: req.SKU, Color: req.Color, Size: req.Size, Price: req.Price, StockQuantity: req.StockQuantity})
	if err != nil {
		productError(c, err, "could not create variant")
		return
	}
	c.JSON(http.StatusCreated, variant)
}

// ListProducts returns published products, optionally filtered by price and sorted.
func (h ProductHandler) ListProducts(c *gin.Context) {
	minPrice, ok := queryInt(c, "min_price")
	if !ok {
		return
	}
	maxPrice, ok := queryInt(c, "max_price")
	if !ok {
		return
	}
	h.listProducts(c, false, false, catalog.ListFilter{MinPrice: minPrice, MaxPrice: maxPrice, Sort: c.Query("sort")})
}

// ListFeaturedProducts returns published featured products.
func (h ProductHandler) ListFeaturedProducts(c *gin.Context) {
	h.listProducts(c, true, false, catalog.ListFilter{})
}

// AdminListProducts returns the owner catalog, including unpublished products.
func (h ProductHandler) AdminListProducts(c *gin.Context) {
	h.listProducts(c, false, true, catalog.ListFilter{})
}

// queryInt reads an optional whole-number query value; a missing value is 0, and text like "abc" gets a 400.
func queryInt(c *gin.Context, name string) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return 0, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": name + " must be a whole number"})
		return 0, false
	}
	return value, true
}

// listProducts delegates filtering and maps storage failures to safe responses.
func (h ProductHandler) listProducts(c *gin.Context, featuredOnly, includeInactive bool, filter catalog.ListFilter) {
	products, err := h.Service.List(c.Request.Context(), featuredOnly, includeInactive, filter)
	if err != nil {
		productError(c, err, "could not load products")
		return
	}
	c.JSON(http.StatusOK, products)
}

// GetProduct returns one published product by its slug.
func (h ProductHandler) GetProduct(c *gin.Context) {
	product, err := h.Service.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		productError(c, err, "could not load product")
		return
	}
	c.JSON(http.StatusOK, product)
}

// CreateProduct accepts an owner product and delegates its rules and persistence.
func (h ProductHandler) CreateProduct(c *gin.Context) {
	h.saveProduct(c, 0)
}

// UpdateProduct accepts an owner edit after parsing its product ID.
func (h ProductHandler) UpdateProduct(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	h.saveProduct(c, id)
}

// saveProduct parses transport input and maps catalog errors to HTTP statuses.
func (h ProductHandler) saveProduct(c *gin.Context, id int) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var req productRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product data"})
		return
	}
	product, err := h.Service.Save(c.Request.Context(), id, catalog.ProductInput{
		CategoryID: req.CategoryID, Name: req.Name, Slug: req.Slug, Description: req.Description, SKU: req.SKU,
		Price: req.Price, OldPrice: req.OldPrice, StockQuantity: req.StockQuantity,
		ExpectedStockQuantity: req.ExpectedStockQuantity,
		BrandTier:             req.BrandTier, FrameType: req.FrameType, Gender: req.Gender,
		Material: req.Material, Color: req.Color, Size: req.Size,
		LensWidth: req.LensWidth, BridgeWidth: req.BridgeWidth, TempleLength: req.TempleLength,
		ImageURL: req.ImageURL, TryOnImageURL: req.TryOnImageURL,
		IsFeatured: req.IsFeatured, IsActive: req.IsActive,
	})
	if err != nil {
		productError(c, err, "could not save product")
		return
	}
	c.JSON(http.StatusOK, product)
}

// DeleteProduct removes a product after the owner route has authenticated the caller.
func (h ProductHandler) DeleteProduct(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	if err := h.Service.Delete(c.Request.Context(), id); err != nil {
		productError(c, err, "could not delete product")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// PatchProductStatus changes publication without changing product price or stock.
func (h ProductHandler) PatchProductStatus(c *gin.Context) {
	id, ok := productID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req struct {
		IsActive *bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.IsActive == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_active is required"})
		return
	}
	if err := h.Service.SetActive(c.Request.Context(), id, *req.IsActive); err != nil {
		productError(c, err, "could not update status")
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": true})
}

// productID rejects malformed route IDs before catalog work begins.
func productID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return 0, false
	}
	return id, true
}

// productError exposes expected catalog outcomes while keeping storage details private.
func productError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, catalog.ErrInvalidFilter):
		c.JSON(http.StatusBadRequest, gin.H{"error": "price range or sort is not valid"})
	case errors.Is(err, catalog.ErrUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database is not connected"})
	case errors.Is(err, catalog.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
	case errors.Is(err, catalog.ErrStaleStock):
		c.JSON(http.StatusConflict, gin.H{"error": "product stock changed; reload and retry"})
	case errors.Is(err, catalog.ErrCategoryRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please select a category."})
	case errors.Is(err, catalog.ErrNamePriceRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "product name and price are required"})
	case errors.Is(err, catalog.ErrNegativeStock):
		c.JSON(http.StatusBadRequest, gin.H{"error": "stock quantity cannot be negative"})
	case errors.Is(err, catalog.ErrExpectedStockMissing):
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected stock quantity is required"})
	case errors.Is(err, catalog.ErrInvalidVariant):
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid SKU, color, size, price and stock are required"})
	case errors.Is(err, catalog.ErrDuplicateVariant):
		c.JSON(http.StatusConflict, gin.H{"error": "SKU or color/size is already in use"})
	case errors.Is(err, catalog.ErrHasVariants):
		c.JSON(http.StatusConflict, gin.H{"error": "remove variants before deleting this product"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": fallback})
	}
}
