package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	// Database (SQLite or PostgreSQL)
	DBDriver     string // "sqlite" or "postgres"
	DBPath       string // for sqlite
	DBDSN        string // for postgres
	// Auth & Security
	JWTSecret     string
	EncryptionKey string
	// Currency & Localization
	DefaultCurrency string // e.g. "INR", "USD", "EUR", "GBP"
	// Email IMAP fallback settings
	IMAPServer     string
	IMAPUsername   string
	IMAPPassword   string
	IMAPFolder     string
	FetchBatchSize int
	LookbackDays   int
	// Concurrency & Scale
	SyncWorkers int
	// Local LLM settings
	OllamaBaseURL       string
	OllamaModel         string
	LLMEnabled          bool
	LLMTimeoutSeconds   int
	// Scheduler settings
	CronSchedule string
	CronEnabled  bool
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[CONFIG] No .env file found or error reading, using environment defaults")
	}

	return &Config{
		Port:              getEnv("PORT", "8080"),
		DBDriver:          getEnv("DB_DRIVER", "sqlite"),
		DBPath:            getEnv("DB_PATH", "./paytrack.db"),
		DBDSN:             getEnv("DB_DSN", getEnv("DATABASE_URL", "host=localhost user=postgres password=postgres dbname=paytrack port=5432 sslmode=disable")),
		JWTSecret:         getEnv("JWT_SECRET", "paytrack-jwt-secret-key-change-in-production"),
		EncryptionKey:     getEnv("ENCRYPTION_KEY", "paytrack-encryption-key-32-chars-!"),
		DefaultCurrency:   getEnv("DEFAULT_CURRENCY", "INR"), // Default to INR / Rupees
		IMAPServer:        getEnv("IMAP_SERVER", "imap.gmail.com:993"),
		IMAPUsername:      getEnv("IMAP_USERNAME", ""),
		IMAPPassword:      getEnv("IMAP_PASSWORD", ""),
		IMAPFolder:        getEnv("IMAP_FOLDER", "INBOX"),
		FetchBatchSize:    getEnvInt("FETCH_BATCH_SIZE", 50),
		LookbackDays:      getEnvInt("LOOKBACK_DAYS", 7),
		SyncWorkers:       getEnvInt("SYNC_WORKERS", 5),
		OllamaBaseURL:     getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaModel:       getEnv("OLLAMA_MODEL", "qwen2.5:3b"),
		LLMEnabled:        getEnvBool("LLM_ENABLED", true),
		LLMTimeoutSeconds: getEnvInt("LLM_TIMEOUT_SECONDS", 30),
		CronSchedule:      getEnv("CRON_SCHEDULE", "0 0,6,12,18 * * *"),
		CronEnabled:       getEnvBool("CRON_ENABLED", true),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}
