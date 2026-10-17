package routes

import (
	"bongkey/backend/internal/auth"
	"bongkey/backend/internal/config"
	"bongkey/backend/internal/database"
	"bongkey/backend/internal/handlers"
	"bongkey/backend/internal/middleware"
	"bongkey/backend/internal/orders"

	"github.com/gin-gonic/gin"
)

// SetupRouter registers public and admin routes for later HTTP requests.
func SetupRouter(cfg config.Config) *gin.Engine {
	router := gin.Default()
	router.Use(middleware.CORS())
	router.Static("/uploads", cfg.ProjectRoot+"/backend/uploads")
	orderAccess := handlers.OrderAccessHandler{Service: orders.AccessService{
		Reader: orders.PostgresReader{DB: database.DB},
	}}
	checkout := handlers.CheckoutHandler{Service: orders.CheckoutService{
		Store: orders.PostgresCheckoutStore{DB: database.DB},
	}}
	adminService := auth.Service{Repository: auth.PostgresRepository{DB: database.DB}, JWTSecret: cfg.JWTSecret}
	adminHandler := handlers.AdminHandler{Service: adminService}

	router.GET("/health", handlers.Health)
	router.POST("/api/admin/login", adminHandler.Login)

	router.GET("/api/products", handlers.ListProducts)
	router.GET("/api/products/featured", handlers.ListFeaturedProducts)
	router.GET("/api/products/:slug", handlers.GetProduct)
	router.GET("/api/categories", handlers.ListCategories)
	router.POST("/api/orders", checkout.CreateOrder)
	router.GET("/api/orders/:id", orderAccess.GetOrder)

	admin := router.Group("/api/admin")
	admin.Use(middleware.AdminAuth(adminService))
	uploads := handlers.UploadHandler{RootDir: cfg.ProjectRoot}
	admin.GET("/me", adminHandler.Me)
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
	admin.GET("/orders/:id", orderAccess.GetAdminOrder)

	return router
}
