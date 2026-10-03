package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/paytrack/backend/internal/api"
	"github.com/paytrack/backend/internal/config"
	"github.com/paytrack/backend/internal/database"
	"github.com/paytrack/backend/internal/merchant"
	"github.com/paytrack/backend/internal/scheduler"
	"github.com/paytrack/backend/internal/service"
)

func main() {
	log.Println("==================================================")
	log.Println("       PayTrack Scalable Multi-User Backend       ")
	log.Println("    Automated Email Payment & Expense Tracker    ")
	log.Println("==================================================")

	// 1. Load configuration
	cfg := config.LoadConfig()

	// 2. Initialize Database (SQLite or PostgreSQL)
	db, err := database.InitDB(cfg.DBDriver, cfg.DBPath, cfg.DBDSN)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize database: %v\n", err)
	}

	// 3. Initialize Dynamic Merchant Registry
	merchant.InitRegistry(filepath.Dir(cfg.DBPath))

	// 4. Initialize Services
	syncService := service.NewSyncService(cfg, db)
	analyticsService := service.NewAnalyticsService(db, syncService)

	// 4. Initialize & Start 4x Daily Cron Scheduler
	cronScheduler := scheduler.NewCronScheduler(cfg, syncService)
	if err := cronScheduler.Start(); err != nil {
		log.Printf("[WARN] Failed to start cron scheduler: %v\n", err)
	}
	defer cronScheduler.Stop()

	// 5. Setup Router and HTTP Server with JWT authentication
	router := api.SetupRouter(cfg, db, syncService, analyticsService)
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 6. Start server in goroutine
	go func() {
		log.Printf("[SERVER] PayTrack API listening on http://localhost:%s\n", cfg.Port)
		log.Printf("[SERVER] Web Admin Dashboard: http://localhost:%s/admin\n", cfg.Port)
		log.Printf("[SERVER] Database:          %s\n", cfg.DBDriver)
		log.Printf("[SERVER] Worker Pool:       %d concurrent sync workers\n", cfg.SyncWorkers)
		log.Printf("[SERVER] Health Check:      GET  http://localhost:%s/health\n", cfg.Port)
		log.Printf("[SERVER] Signup:            POST http://localhost:%s/api/v1/auth/signup\n", cfg.Port)
		log.Printf("[SERVER] Login:             POST http://localhost:%s/api/v1/auth/login\n", cfg.Port)
		log.Printf("[SERVER] Connect Mailbox:   POST http://localhost:%s/api/v1/accounts\n", cfg.Port)
		log.Printf("[SERVER] Dashboard Stats:   GET  http://localhost:%s/api/v1/analytics/dashboard\n", cfg.Port)
		log.Printf("[SERVER] Monthly Report:    GET  http://localhost:%s/api/v1/analytics/monthly?year=2026&month=10\n", cfg.Port)
		log.Printf("[SERVER] Expenses List:     GET  http://localhost:%s/api/v1/expenses\n", cfg.Port)
		log.Printf("[SERVER] Trigger Sync:      POST http://localhost:%s/api/v1/sync\n", cfg.Port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server error: %v\n", err)
		}
	}()

	// 7. Wait for graceful shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[SERVER] Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[SERVER] Error during server shutdown: %v\n", err)
	}

	log.Println("[SERVER] PayTrack Service stopped cleanly.")
	fmt.Println("Goodbye!")
}
