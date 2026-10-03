package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/paytrack/backend/internal/config"
	"github.com/paytrack/backend/internal/service"
	"github.com/paytrack/backend/internal/web"
	"gorm.io/gorm"
)

func SetupRouter(cfg *config.Config, db *gorm.DB, syncService *service.SyncService, analyticsService *service.AnalyticsService) http.Handler {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration for mobile apps and web frontends
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Web Admin Dashboard UI
	r.Get("/admin", web.ServeAdminDashboard)

	// Instantiate handlers
	authHandler := NewAuthHandler(db, cfg)
	accountHandler := NewAccountHandler(db, cfg, syncService)
	expenseHandler := NewExpenseHandler(db)
	analyticsHandler := NewAnalyticsHandler(analyticsService)
	syncHandler := NewSyncHandler(db, syncService)
	adminHandler := NewAdminHandler(db, cfg, syncService)

	// API V1 Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]string{
				"status":  "ok",
				"version": "v1",
				"time":    time.Now().UTC().Format(time.RFC3339),
			})
		})

		// Public Authentication routes
		r.Route("/auth", func(r chi.Router) {
			r.Post("/signup", authHandler.Signup)
			r.Post("/login", authHandler.Login)
		})

		// Admin monitoring endpoints (for the admin dashboard)
		r.Route("/admin", func(r chi.Router) {
			r.Get("/stats", adminHandler.GetSystemStats)
			r.Get("/users", adminHandler.GetUsersList)
			r.Get("/sync-logs", adminHandler.GetGlobalSyncLogs)
			r.Get("/transactions", adminHandler.GetGlobalTransactions)
			r.Post("/sync-all", adminHandler.TriggerGlobalSync)
		})

		// Protected routes requiring JWT Authentication
		r.Group(func(protected chi.Router) {
			protected.Use(AuthMiddleware(cfg.JWTSecret))

			// Current user info
			protected.Get("/auth/me", authHandler.Me)

			// Connected Email Inboxes
			protected.Route("/accounts", func(r chi.Router) {
				r.Get("/", accountHandler.ListAccounts)
				r.Post("/", accountHandler.AddAccount)
				r.Delete("/{id}", accountHandler.DeleteAccount)
			})

			// Expenses CRUD
			protected.Route("/expenses", func(r chi.Router) {
				r.Get("/", expenseHandler.ListExpenses)
				r.Post("/", expenseHandler.CreateManualExpense)
				r.Get("/{id}", expenseHandler.GetExpenseByID)
				r.Put("/{id}", expenseHandler.UpdateExpense)
				r.Delete("/{id}", expenseHandler.DeleteExpense)
			})

			// Analytics & Dashboard (for mobile graphs/summaries)
			protected.Route("/analytics", func(r chi.Router) {
				r.Get("/dashboard", analyticsHandler.GetDashboardSummary)
				r.Get("/monthly", analyticsHandler.GetMonthlyReport)
			})

			// Email Sync operations
			protected.Route("/sync", func(r chi.Router) {
				r.Post("/", syncHandler.TriggerManualSync)
				r.Get("/status", syncHandler.GetSyncStatus)
				r.Get("/logs", syncHandler.GetSyncLogs)
			})
		})
	})

	return r
}
