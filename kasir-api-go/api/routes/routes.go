package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"kasir-api-go/api/handlers"
	"kasir-api-go/db"
	"kasir-api-go/middleware"
)

func SetupRoutes(router *gin.Engine, queries *db.Queries, pool *pgxpool.Pool, rdb *redis.Client, draftRdb *redis.Client) {
	// Initialize handlers
	wsManager := handlers.NewWebSocketManager()
	userHandler := handlers.NewUserHandler(queries)
	productHandler := handlers.NewProductHandler(queries, wsManager)
	transactionHandler := handlers.NewTransactionHandler(queries, pool)
	authHandler := handlers.NewAuthHandler(queries, pool)
	adminHandler := handlers.NewAdminHandler(queries, pool, draftRdb)
	storeHandler := handlers.NewStoreHandler(queries)
	reportHandler := handlers.NewReportHandler(queries)
	feedbackHandler := handlers.NewFeedbackHandler(queries)
	subscriptionHandler := handlers.NewSubscriptionHandler(queries)
	offHandler := handlers.NewOFFHandler(queries, draftRdb)

	authMw := handlers.AuthMiddleware(queries)

	// Root route to prevent 404 on base domain
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "POS Kedai API is running. Access endpoints under /api",
		})
	})

	api := router.Group("/api")
	{
		// Global security headers (HSTS, nosniff, XSS protect) — murah, defence-in-depth.
		api.Use(middleware.SecurityHeaders())

		// Rate limit global: 120 req/menit per IP+path.
		// Auth endpoint punya limit lebih ketat di bawah.
		api.Use(middleware.RateLimit(rdb, 120, time.Minute))
		// Health check
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// User routes
		users := api.Group("/users")
		{
			// Pembuatan user & list semua user = privilege tinggi → hanya admin.
			users.POST("/", middleware.AdminOnly(), userHandler.CreateUser)
			users.GET("/", middleware.AdminOnly(), userHandler.ListUsers)
			users.GET("/:id", userHandler.GetUser)
			usersAuth := users.Group("/store")
			usersAuth.Use(authMw)
			usersAuth.GET("/", userHandler.ListStoreUsers)
			usersAuth.POST("/add-employee", userHandler.AddStoreEmployee)
			usersAuth.PUT("/profile", userHandler.UpdateProfile)
			usersAuth.DELETE("/employees/:id", userHandler.DeleteStoreEmployee)
			users.PUT("/fcm-token", authMw, userHandler.UpdateFCMToken)
		}

		// Product routes
		products := api.Group("/products")
		{
			products.GET("/stock-report", authMw, productHandler.GetStockReport)
			products.POST("/master", authMw, productHandler.CreateMasterProduct)
			products.GET("/master/:id", productHandler.GetMasterProduct)
			products.GET("/master", productHandler.ListMasterProducts)
			products.GET("/master-status", adminHandler.GetMasterProductStatus)

			products.POST("/store", authMw, productHandler.CreateStoreProduct)
			products.GET("/store/:id", productHandler.GetStoreProduct)
			products.GET("/store", productHandler.ListStoreProducts)
			products.PUT("/store/:id/stock", authMw, productHandler.AddStoreProductStock)

			products.POST("/pending", authMw, productHandler.SubmitPendingProduct)
			products.GET("/pending", productHandler.ListPendingProducts)

			products.DELETE("/store/:id", authMw, productHandler.DeleteStoreProductSpecific)
			products.DELETE("/:id", authMw, productHandler.DeleteProduct)
			products.PUT("/:id", authMw, productHandler.UpdateProduct)
		}

		// Report routes
		reports := api.Group("/reports")
		reports.Use(authMw)
		{
			reports.POST("/", reportHandler.SubmitReport)
			reports.GET("/", reportHandler.GetStoreReports)
			reports.DELETE("/:id", reportHandler.DeleteReport)
		}

		// Transaction routes
		transactions := api.Group("/transactions")
		transactions.Use(authMw)
		{
			transactions.POST("/", transactionHandler.CreateTransaction)
			transactions.GET("/dashboard", transactionHandler.GetDashboardStats)
			transactions.GET("/", transactionHandler.GetAllTransactions)
		}

		// Store routes
		stores := api.Group("/stores")
		stores.Use(authMw)
		{
			stores.PUT("/update", storeHandler.UpdateStore)
			stores.POST("/upload-logo", storeHandler.UploadStoreLogo)
		}

		// WebSocket routes
		api.GET("/ws", wsManager.HandleConnections)

		// Auth (public, rawan brute-force) — limit ketat per IP+path.
		api.POST("/login", middleware.RateLimit(rdb, 10, time.Minute), authHandler.Login) // Used by Admin
		api.POST("/auth/login", middleware.RateLimit(rdb, 10, time.Minute), authHandler.Login) // Used by Store
		api.POST("/auth/register-store", middleware.RateLimit(rdb, 5, time.Minute), authHandler.RegisterStore)
		api.POST("/auth/verify-otp", middleware.RateLimit(rdb, 10, time.Minute), authHandler.VerifyOTP)
		api.POST("/auth/resend-otp", middleware.RateLimit(rdb, 5, time.Minute), authHandler.ResendOTP)
		api.POST("/auth/forgot-password", middleware.RateLimit(rdb, 5, time.Minute), authHandler.ForgotPassword)
		api.POST("/auth/verify-forgot-otp", middleware.RateLimit(rdb, 10, time.Minute), authHandler.VerifyForgotOTP)
		api.POST("/auth/reset-password", middleware.RateLimit(rdb, 5, time.Minute), authHandler.ResetPassword)
		authGroup := api.Group("/auth")
		authGroup.Use(authMw)
		authGroup.POST("/switch-user", authHandler.SwitchUser)

		// Subscription routes for store
		subscriptions := api.Group("/subscriptions")
		subscriptions.Use(authMw)
		{
			subscriptions.GET("/plans", subscriptionHandler.GetPlans)
			subscriptions.GET("/payment-channels", subscriptionHandler.GetPaymentChannels)
			subscriptions.GET("/payment-instructions", subscriptionHandler.GetPaymentInstructions)
			subscriptions.GET("/fee-calculator", subscriptionHandler.GetFeeCalculator)
			subscriptions.POST("/checkout", subscriptionHandler.Checkout)
			subscriptions.GET("/transactions/:reference", subscriptionHandler.GetTransactionByRef)
		}

		// Tripay Callback Webhook (Public, verified via signature)
		api.POST("/tripay/callback", subscriptionHandler.TripayCallback)

		// Admin routes — wajib admin + auth.
		adminRoutes := api.Group("/admin")
		adminRoutes.Use(authMw, middleware.AdminOnly())
		{
			adminRoutes.GET("/dashboard", adminHandler.GetDashboardStats)
			adminRoutes.POST("/products/:id/approve", adminHandler.ApproveProduct)
			adminRoutes.POST("/products/:id/reject", adminHandler.RejectProduct)

			// Admin Store Management
			adminRoutes.GET("/stores", adminHandler.ListStores)
			adminRoutes.GET("/stores/:id", adminHandler.GetStoreDetail)
			adminRoutes.PUT("/stores/:id/pro", adminHandler.UpdateStoreProStatus)
			adminRoutes.PUT("/stores/:id/block", adminHandler.ToggleStoreBlock)
			adminRoutes.DELETE("/stores/:id", adminHandler.DeleteStore)

			// Admin Subscription Management
			adminRoutes.GET("/subscription-plans", subscriptionHandler.AdminListPlans)
			adminRoutes.POST("/subscription-plans", subscriptionHandler.AdminCreatePlan)
			adminRoutes.PUT("/subscription-plans/:id", subscriptionHandler.AdminUpdatePlan)
			adminRoutes.DELETE("/subscription-plans/:id", subscriptionHandler.AdminDeletePlan)
			adminRoutes.GET("/subscription-transactions", subscriptionHandler.AdminListTransactions)

			// Open Food Facts Draft Management (Staging di Redis DB 1)
			adminRoutes.POST("/off/fetch", offHandler.FetchFromOFF)
			adminRoutes.GET("/off/drafts", offHandler.ListDrafts)
			adminRoutes.POST("/off/drafts/approve", offHandler.ApproveDraft)
			adminRoutes.DELETE("/off/drafts/:barcode", offHandler.RejectDraft)

			// Master Product Global Toggle
			adminRoutes.GET("/master-product-status", adminHandler.GetMasterProductStatus)
			adminRoutes.PUT("/master-product-status", adminHandler.ToggleMasterProduct)
		}

		// Feedback route
		api.POST("/feedback", authMw, feedbackHandler.SendFeedback)

		// Upload route
		api.POST("/upload", handlers.UploadImage)
	}

	// Serve static files for uploads
	router.Static("/uploads", "./uploads")
}
