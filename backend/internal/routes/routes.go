package routes

import (
	"fmt"
	"strings"

	"bongkey/backend/internal/audit"
	"bongkey/backend/internal/auth"
	"bongkey/backend/internal/callbacks"
	"bongkey/backend/internal/catalog"
	"bongkey/backend/internal/config"
	"bongkey/backend/internal/customers"
	"bongkey/backend/internal/dashboard"
	"bongkey/backend/internal/database"
	"bongkey/backend/internal/delivery"
	"bongkey/backend/internal/emailotp"
	"bongkey/backend/internal/handlers"
	"bongkey/backend/internal/lens"
	"bongkey/backend/internal/mail"
	"bongkey/backend/internal/middleware"
	"bongkey/backend/internal/notifications"
	"bongkey/backend/internal/orders"
	"bongkey/backend/internal/payments"
	"bongkey/backend/internal/returns"
	"bongkey/backend/internal/security"

	"github.com/gin-gonic/gin"
)

// logWithoutQuery formats one access-log line with the request ID and without the query string,
// because admin searches carry customer phone numbers.
func logWithoutQuery(p gin.LogFormatterParams) string {
	path, _, _ := strings.Cut(p.Path, "?")
	id, _ := p.Keys[middleware.RequestIDKey].(string)
	return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %s | %-7s %s\n%s",
		p.TimeStamp.Format("2006/01/02 - 15:04:05"), p.StatusCode, p.Latency, p.ClientIP, id, p.Method, path, p.ErrorMessage)
}

// SetupRouter configures direct-client trust and registers routes for later HTTP requests.
func SetupRouter(cfg config.Config) *gin.Engine {
	router := gin.New()
	// Order matters: RequestID first so both read the ID; the logger wraps Recovery so a recovered panic is still logged as a 500.
	router.Use(middleware.RequestID(), gin.LoggerWithFormatter(logWithoutQuery), middleware.Recovery(gin.DefaultErrorWriter))
	// Direct deployments must ignore forwarded client IP headers until an exact reverse proxy is configured.
	if err := router.SetTrustedProxies(nil); err != nil {
		panic("disable trusted proxies: " + err.Error())
	}
	router.Use(middleware.CORS(cfg.FrontendOrigins))
	router.Static("/uploads", cfg.ProjectRoot+"/backend/uploads")
	// Prescriptions live outside /uploads so the public static route can never serve them.
	prescriptions := lens.PrescriptionStore{Dir: cfg.PrescriptionDir}
	lensReview := handlers.LensReviewHandler{Service: orders.LensReviewService{Store: orders.PostgresLensReviewStore{DB: database.DB}, Files: prescriptions}, Files: prescriptions}
	// The slip prints the same public address as the emails (PUBLIC_SITE_URL, else the first allowed origin).
	orderAccess := handlers.OrderAccessHandler{SiteURL: cfg.SiteURL, Service: orders.AccessService{
		Reader:           orders.PostgresReader{DB: database.DB},
		EmailProofSecret: cfg.JWTSecret,
		// shortcut: shares the login limiter's table (keys are HMACs with their own "order-track-ip" kind, so budgets never mix); give tracking its own table if limits need tuning separately.
		Limiter: auth.PostgresLoginLimiter{DB: database.DB},
	}}
	mailer := mail.New(cfg)
	// Emails are queued in the database after each change commits and sent by the worker that serve starts.
	notifier := notifications.Service{Store: notifications.PostgresStore{DB: database.DB}, Orders: orderAccess.Service, OwnerEmail: cfg.OwnerNotifyEmail, SiteURL: cfg.SiteURL}
	// Same shared limiter table as tracking, with its own "order-numbers-ip" key kind.
	orderAccess.Numbers = orders.OrderNumberService{Orders: orders.PostgresReader{DB: database.DB}, Sender: notifier, Limiter: auth.PostgresLoginLimiter{DB: database.DB}, Secret: cfg.JWTSecret}
	callbackHandler := handlers.CallbackHandler{Service: callbacks.Service{Store: callbacks.PostgresStore{DB: database.DB}, Limiter: auth.PostgresLoginLimiter{DB: database.DB}, Notifier: notifier, Secret: cfg.JWTSecret}}
	returnsHandler := handlers.ReturnsHandler{Orders: orderAccess.Service,
		Service: returns.Service{Store: returns.PostgresStore{DB: database.DB}, Photos: lens.PrescriptionStore{Dir: cfg.ReturnPhotoDir}, Notifier: notifier}}
	paymentService := payments.Service{Store: payments.PostgresStore{DB: database.DB}, Notifier: notifier}
	paymentHandler := handlers.PaymentHandler{Service: paymentService, Orders: orderAccess.Service}
	checkout := handlers.CheckoutHandler{Service: orders.CheckoutService{
		Store:               orders.PostgresCheckoutStore{DB: database.DB},
		EmailProofSecret:    cfg.JWTSecret,
		OrderTokenSecret:    cfg.JWTSecret,
		DeliveryFeeProvider: delivery.Service{Store: delivery.PostgresRepository{DB: database.DB}},
		Prescriptions:       prescriptions,
		PaymentOptions:      paymentService,
		Notifier:            notifier,
		// Same shared limiter table as tracking, with its own "checkout-phone" and "checkout-ip" key kinds.
		Limiter: auth.PostgresLoginLimiter{DB: database.DB},
	}}
	deliverySettings := handlers.DeliverySettingsHandler{Service: delivery.Service{Store: delivery.PostgresRepository{DB: database.DB}}}
	emailCodes := handlers.EmailOTPHandler{Service: emailotp.Service{
		Store:    emailotp.PostgresStore{DB: database.DB},
		Sender:   emailotp.CodeMailer{Mail: mailer},
		Secret:   cfg.JWTSecret,
		TestCode: cfg.EmailTestCode,
	}}
	adminService := auth.Service{Repository: auth.PostgresRepository{DB: database.DB}, Limiter: auth.PostgresLoginLimiter{DB: database.DB}, JWTSecret: cfg.JWTSecret}
	adminHandler := handlers.AdminHandler{Service: adminService}
	products := handlers.ProductHandler{Service: catalog.ProductService{
		Repository: catalog.PostgresProductRepository{DB: database.DB},
	}}
	stock := handlers.StockHandler{Service: catalog.StockService{Store: catalog.PostgresProductRepository{DB: database.DB}}}
	lensPackages := handlers.LensHandler{Service: lens.Service{Repository: lens.PostgresRepository{DB: database.DB}}}
	categories := handlers.CategoryHandler{Service: catalog.CategoryService{Repository: catalog.PostgresCategoryRepository{DB: database.DB}}}
	customerService := customers.Service{
		Store: customers.PostgresStore{DB: database.DB},
		// shortcut: shares the login limiter's table (HMAC keys with their own "customer-link" kind); split it if limits need separate tuning.
		Limiter: auth.PostgresLoginLimiter{DB: database.DB},
		Secret:  cfg.JWTSecret,
	}
	if cfg.GoogleClientID != "" {
		customerService.Google = security.GoogleVerifier{ClientID: cfg.GoogleClientID, Keys: &security.GoogleKeys{}}
	}
	customerAccount := handlers.CustomerHandler{Service: customerService, SecureCookie: cfg.AppEnv != "development"}
	orderList := handlers.OrderListHandler{Reader: orders.PostgresReader{DB: database.DB}}
	orderFulfillment := handlers.OrderFulfillmentHandler{Service: orders.FulfillmentService{
		Store:    orders.PostgresFulfillmentStore{DB: database.DB},
		Notifier: notifier,
	}, Tracking: orders.TrackingService{Store: orders.PostgresFulfillmentStore{DB: database.DB}},
		Emails: orders.EmailConfirmationService{Store: orders.PostgresFulfillmentStore{DB: database.DB}, Notifier: notifier},
		Notes:  orders.StaffNoteService{Store: orders.PostgresFulfillmentStore{DB: database.DB}}}

	router.GET("/health", handlers.Health)
	router.POST("/api/admin/login", adminHandler.Login)

	router.GET("/api/products", products.ListProducts)
	router.GET("/api/products/featured", products.ListFeaturedProducts)
	router.GET("/api/products/:slug", products.GetProduct)
	router.GET("/api/products/:slug/lens-packages", lensPackages.ListForProduct)
	router.GET("/api/categories", categories.ListCategories)
	router.GET("/api/delivery-settings", deliverySettings.Get)
	// OptionalCustomer never blocks checkout; it only lets a signed-in customer's order show in "My orders".
	router.POST("/api/orders", middleware.OptionalCustomer(customerService), checkout.CreateOrder)
	router.POST("/api/track", orderAccess.TrackOrder)
	router.POST("/api/track/order-numbers", orderAccess.SendOrderNumbers)
	router.POST("/api/callback-requests", callbackHandler.Create)
	router.POST("/api/orders/:id/returns", returnsHandler.CreateFromOrderPage)
	router.POST("/api/track/returns", returnsHandler.CreateFromTrack)
	router.GET("/api/orders/:id", orderAccess.GetOrder)
	router.GET("/api/orders/:id/invoice.pdf", orderAccess.GetInvoice)
	router.GET("/api/orders/:id/slip.pdf", orderAccess.GetSlip)
	router.GET("/api/payment-options", paymentHandler.PaymentOptions)
	router.GET("/api/orders/:id/payments", paymentHandler.GuestPayments)
	router.POST("/api/orders/:id/payments", paymentHandler.SubmitPayment)
	router.POST("/api/email-otp/request", emailCodes.Request)
	router.POST("/api/email-otp/verify", emailCodes.Verify)

	// CustomerCSRF guards every customer route, sign-in included; CustomerAuth then needs a session for the rest.
	customer := router.Group("/api/customer", middleware.CustomerCSRF())
	customer.POST("/google", customerAccount.SignIn)
	customer.POST("/login", customerAccount.Login)
	// Create account and forgot password share one step: the email code proves the email, then a password is set.
	customer.POST("/password", customerAccount.SetPassword)
	customer.POST("/logout", customerAccount.Logout)
	signedIn := customer.Group("", middleware.CustomerAuth(customerService))
	signedIn.GET("/me", customerAccount.Me)
	signedIn.PUT("/profile", customerAccount.UpdateProfile)
	signedIn.GET("/orders", customerAccount.Orders)
	signedIn.GET("/link-candidates", customerAccount.LinkCandidates)
	signedIn.POST("/link", customerAccount.Link)

	admin := router.Group("/api/admin")
	admin.Use(middleware.AdminAuth(adminService))
	uploads := handlers.UploadHandler{RootDir: cfg.ProjectRoot}
	admin.GET("/me", adminHandler.Me)
	admin.GET("/audit", handlers.AuditHandler{Reader: audit.Reader{DB: database.DB}}.List)
	admin.POST("/uploads/product-image", uploads.ProductImage)
	admin.POST("/uploads/try-on-mockup", uploads.TryOnMockup)
	admin.GET("/products", products.AdminListProducts)
	admin.POST("/products", products.CreateProduct)
	admin.GET("/products/:id/variants", products.ListVariants)
	admin.GET("/products/:id/stock-history", stock.History)
	admin.GET("/stock/low", stock.LowStock)
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
	admin.PUT("/orders/:id/tracking", orderFulfillment.SetTracking)
	admin.PUT("/orders/:id/email-confirmed", orderFulfillment.SetEmailConfirmed)
	admin.PUT("/orders/:id/note", orderFulfillment.SetStaffNote)
	admin.GET("/todo", handlers.TodoHandler{Reader: dashboard.Reader{DB: database.DB}}.Counts)
	admin.POST("/orders/:id/advance-received", lensReview.MarkAdvanceReceived)
	admin.PATCH("/orders/:id/items/:itemId/prescription-status", lensReview.SetPrescriptionStatus)
	admin.PATCH("/orders/:id/items/:itemId/lens-price", lensReview.SetLensPrice)
	admin.GET("/orders/:id/items/:itemId/prescription", lensReview.Prescription)
	admin.POST("/orders/:id/items/:itemId/prescription", lensReview.AttachPrescription)
	admin.PUT("/delivery-settings", deliverySettings.Update)
	admin.GET("/lens-packages", lensPackages.AdminListPackages)
	admin.POST("/lens-packages", lensPackages.CreatePackage)
	admin.PUT("/lens-packages/:id", lensPackages.UpdatePackage)
	admin.GET("/payments", paymentHandler.AdminPayments)
	admin.POST("/payments/:id/approve", paymentHandler.ApprovePayment)
	admin.POST("/payments/:id/reject", paymentHandler.RejectPayment)
	admin.GET("/orders/:id/payments", paymentHandler.AdminOrderPayments)
	admin.GET("/payment-methods/:code", paymentHandler.PaymentMethod)
	admin.GET("/callback-requests", callbackHandler.AdminList)
	admin.GET("/returns", returnsHandler.AdminList)
	admin.PATCH("/returns/:id", returnsHandler.Update)
	admin.POST("/returns/:id/refund", returnsHandler.RecordRefund)
	admin.GET("/returns/:id/photos/:index", returnsHandler.Photo)
	admin.POST("/callback-requests/:id/done", callbackHandler.MarkDone)
	admin.PUT("/payment-methods/:code", paymentHandler.SavePaymentMethod)

	return router
}
