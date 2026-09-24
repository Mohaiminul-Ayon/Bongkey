package routes

import (
	"bongkey/backend/internal/config"
	"bongkey/backend/internal/handlers"
	"bongkey/backend/internal/middleware"
	"bongkey/backend/internal/orders"
	"database/sql"

	"github.com/gin-gonic/gin"
)

// SetupRouter registers handlers for later requests; middleware runs logger, recovery, then CORS.
func SetupRouter(cfg config.Config, db *sql.DB) *gin.Engine {
	router := gin.Default()
	router.Use(middleware.CORS())
	router.Static("/uploads", cfg.ProjectRoot+"/backend/uploads")

	router.GET("/health", handlers.Health)
	router.POST("/api/admin/login", handlers.AdminHandler{Config: cfg}.Login)

	router.GET("/api/products", handlers.ListProducts)
	router.GET("/api/products/featured", handlers.ListFeaturedProducts)
	router.GET("/api/products/:slug", handlers.GetProduct)
	router.GET("/api/categories", handlers.ListCategories)
	orderHandler := handlers.OrderHandler{Service: orders.NewService(db)}
	router.POST("/api/orders", orderHandler.Create)
	router.GET("/api/orders/:id", orderHandler.Get)

	admin := router.Group("/api/admin")
	admin.Use(middleware.AdminAuth(cfg))
	uploads := handlers.UploadHandler{RootDir: cfg.ProjectRoot}
	admin.GET("/me", handlers.AdminHandler{Config: cfg}.Me)
	admin.POST("/uploads/product-image", uploads.ProductImage)
	admin.POST("/uploads/try-on-mockup", uploads.TryOnMockup)
	admin.GET("/products", handlers.AdminListProducts)
	admin.POST("/products", handlers.CreateProduct)
	admin.PUT("/products/:id", handlers.UpdateProduct)
	admin.DELETE("/products/:id", handlers.DeleteProduct)
	admin.PATCH("/products/:id/status", handlers.PatchProductStatus)
	admin.GET("/categories", handlers.ListCategories)
	admin.POST("/categories", handlers.CreateCategory)
	admin.PUT("/categories/:id", handlers.UpdateCategory)
	admin.DELETE("/categories/:id", handlers.DeleteCategory)
	admin.GET("/orders", handlers.ListOrders)

	return router
}
