package models

type Category struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type Product struct {
	ID              int       `json:"id"`
	ParentProductID int       `json:"parent_product_id"`
	SKU             string    `json:"sku"`
	CategoryID      int       `json:"category_id"`
	CategoryName    string    `json:"category_name"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	Description     string    `json:"description"`
	Price           int       `json:"price"`
	OldPrice        int       `json:"old_price"`
	BrandTier       string    `json:"brand_tier"`
	FrameType       string    `json:"frame_type"`
	Gender          string    `json:"gender"`
	Material        string    `json:"material"`
	Color           string    `json:"color"`
	Size            string    `json:"size"`
	LensWidth       string    `json:"lens_width"`
	BridgeWidth     string    `json:"bridge_width"`
	TempleLength    string    `json:"temple_length"`
	StockQuantity   int       `json:"stock_quantity"`
	ImageURL        string    `json:"image_url"`
	TryOnImageURL   string    `json:"try_on_image_url"`
	IsFeatured      bool      `json:"is_featured"`
	IsActive        bool      `json:"is_active"`
	Variants        []Product `json:"variants,omitempty"`
}
