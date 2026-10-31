package routes

import (
	"fmt"
	"strings"

	"bongkey/backend/internal/auth"
	"bongkey/backend/internal/catalog"
	"bongkey/backend/internal/config"
	"bongkey/backend/internal/database"
	"bongkey/backend/internal/delivery"
	"bongkey/backend/internal/emailotp"
	"bongkey/backend/internal/handlers"
	"bongkey/backend/internal/middleware"
	"bongkey/backend/internal/orders"

	"github.com/gin-gonic/gin"
)

// logWithoutQuery formats one access-log line without the query string, because admin searches carry customer phone numbers.
func logWithoutQuery(p gin.LogFormatterParams) string {
	path, _, _ := strings.Cut(p.Path, "?")
	return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %s\n%s",
		p.TimeStamp.Format("2006/01/02 - 15:04:05"), p.StatusCode, p.Latency, p.ClientIP, p.Method, path, p.ErrorMessage)
}

// SetupRouter configures direct-client trust and registers routes for later HTTP requests.
func SetupRouter(cfg config.Config) *gin.Engine {
	router := gin.New()
	router.Use(gin.LoggerWithFormatter(logWithoutQuery), gin.Recovery())
	// Direct deployments must ignore forwarded client IP headers until an exact reverse proxy is configured.
	if err := router.SetTrustedProxies(nil); err != nil {
		panic("disable trusted proxies: " + err.Error())
	}
	router.Use(middleware.CORS())
	router.Static("/uploads", cfg.ProjectRoot+"/backend/uploads")
	orderAccess := handlers.OrderAccessHandler{Service: orders.AccessService{
		Reader:           orders.PostgresReader{DB: database.DB},
		EmailProofSecret: cfg.JWTSecret,
	}}
	checkout := handlers.CheckoutHandler{Service: orders.CheckoutService{
		Store:               orders.PostgresCheckoutStore{DB: database.DB},
		EmailProofSecret:    cfg.JWTSecret,
		OrderTokenSecret:    cfg.JWTSecret,
		DeliveryFeeProvider: delivery.Service{Store: delivery.PostgresRepository{DB: database.DB}},
	}}
	deliverySettings := handlers.DeliverySettingsHandler{Service: delivery.Service{Store: delivery.PostgresRepository{DB: database.DB}}}
	emailCodes := handlers.EmailOTPHandler{Service: emailotp.Service{
		Store:  emailotp.PostgresStore{DB: database.DB},
		Sender: emailotp.GmailSender{From: cfg.GmailFrom, AppPassword: cfg.GmailAppPassword},
		Secret: cfg.JWTSecret,
	}}
	adminService := auth.Service{Repository: auth.PostgresRepository{DB: database.DB}, Limiter: auth.PostgresLoginLimiter{DB: database.DB}, JWTSecret: cfg.JWTSecret}
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
	router.GET("/api/delivery-settings", deliverySettings.Get)
	router.POST("/api/orders", checkout.CreateOrder)
	router.GET("/api/orders/:id", orderAccess.GetOrder)
	router.GET("/api/orders/:id/invoice.pdf", orderAccess.GetInvoice)
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
	admin.GET("/products/:id/variants", products.ListVariants)
	admin.POST("/products/:id/variants", products.CreateVariant)
	admin.PUT("/products/:id", products.UpdateProduct)
	admin.DELETE("/products/:id", products.DeleteProduct)
	admin.PATCH("/products/:id/status", products.PatchProductStatus)
	admin.GET("/categories", categories.ListCategories)
	admin.POST("/categories", categories.CreateCategory)
	admin.PUT("/categories/:id", categories.UpdateCategory)
	admin.DELETE("/categories/:id", categories.DeleteCategory)
	admin.GET("/orders", orderList.ListOrders)
	admin.GET("/orders/:id", orderAccess.GetAdminOrder)
	admin.GET("/orders/:id/invoice.pdf", orderAccess.GetAdminInvoice)
	admin.PATCH("/orders/:id/status", orderFulfillment.ChangeStatus)
	admin.PUT("/delivery-settings", deliverySettings.Update)

	return router
}
