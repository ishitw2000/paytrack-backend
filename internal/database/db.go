package database

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/paytrack/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB initializes SQLite or PostgreSQL depending on the driver configuration
func InitDB(driver, dbPath, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	if strings.ToLower(driver) == "postgres" || strings.ToLower(driver) == "postgresql" {
		log.Printf("[DATABASE] Connecting to PostgreSQL database...\n")
		dialector = postgres.Open(dsn)
	} else {
		log.Printf("[DATABASE] Connecting to SQLite database at: %s\n", dbPath)
		dialector = sqlite.Open(dbPath)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(50)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	// Auto-migrate schema tables
	err = db.AutoMigrate(
		&models.User{},
		&models.EmailAccount{},
		&models.Expense{},
		&models.SyncLog{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	DB = db
	log.Printf("[DATABASE] %s database connected and migrated successfully.\n", strings.ToUpper(driver))
	return db, nil
}
