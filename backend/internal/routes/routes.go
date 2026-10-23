package routes

import (
	"bongkey/backend/internal/auth"
	"bongkey/backend/internal/catalog"
	"bongkey/backend/internal/config"
	"bongkey/backend/internal/database"
	"bongkey/backend/internal/emailotp"
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
		Reader:           orders.PostgresReader{DB: database.DB},
		EmailProofSecret: cfg.JWTSecret,
	}}
	checkout := handlers.CheckoutHandler{Service: orders.CheckoutService{
		Store:            orders.PostgresCheckoutStore{DB: database.DB},
		EmailProofSecret: cfg.JWTSecret,
		OrderTokenSecret: cfg.JWTSecret,
	}}
	emailCodes := handlers.EmailOTPHandler{Service: emailotp.Service{
		Store:  emailotp.PostgresStore{DB: database.DB},
		Sender: emailotp.GmailSender{From: cfg.GmailFrom, AppPassword: cfg.GmailAppPassword},
		Secret: cfg.JWTSecret,
	}}
	adminService := auth.Service{Repository: auth.PostgresRepository{DB: database.DB}, JWTSecret: cfg.JWTSecret}
	adminHandler := handlers.AdminHandler{Service: adminService}
	products := handlers.ProductHandler{Service: catalog.ProductService{
		Repository: catalog.PostgresProductRepository{DB: database.DB},
	}}
	categories := handlers.CategoryHandler{Service: catalog.CategoryService{Repository: catalog.PostgresCategoryRepository{DB: database.DB}}}
	orderList := handlers.OrderListHandler{Reader: orders.PostgresReader{DB: database.DB}}
	orderFulfillment := handlers.OrderFulfillmentHandler{Service: orders.FulfillmentService{
		Store: orders.PostgresFulfillmentStore{DB: database.DB},
	}}

	router.GET("/health", handlers.Health)
	router.POST("/api/admin/login", adminHandler.Login)

	router.GET("/api/products", products.ListProducts)
	router.GET("/api/products/featured", products.ListFeaturedProducts)
	router.GET("/api/products/:slug", products.GetProduct)
	router.GET("/api/categories", categories.ListCategories)
	router.POST("/api/orders", checkout.CreateOrder)
	router.GET("/api/orders/:id", orderAccess.GetOrder)
	router.POST("/api/email-otp/request", emailCodes.Request)
	router.POST("/api/email-otp/verify", emailCodes.Verify)

	admin := router.Group("/api/admin")
	admin.Use(middleware.AdminAuth(adminService))
	uploads := handlers.UploadHandler{RootDir: cfg.ProjectRoot}
	admin.GET("/me", adminHandler.Me)
	admin.POST("/uploads/product-image", uploads.ProductImage)
	admin.POST("/uploads/try-on-mockup", uploads.TryOnMockup)
	admin.GET("/products", products.AdminListProducts)
	admin.POST("/products", products.CreateProduct)
	admin.PUT("/products/:id", products.UpdateProduct)
	admin.DELETE("/products/:id", products.DeleteProduct)
	admin.PATCH("/products/:id/status", products.PatchProductStatus)
	admin.GET("/categories", categories.ListCategories)
	admin.POST("/categories", categories.CreateCategory)
	admin.PUT("/categories/:id", categories.UpdateCategory)
	admin.DELETE("/categories/:id", categories.DeleteCategory)
	admin.GET("/orders", orderList.ListOrders)
	admin.GET("/orders/:id", orderAccess.GetAdminOrder)
	admin.PATCH("/orders/:id/status", orderFulfillment.ChangeStatus)

	return router
}
