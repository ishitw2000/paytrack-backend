package api

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/paytrack/backend/internal/config"
	"github.com/paytrack/backend/internal/models"
	"github.com/paytrack/backend/internal/service"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db          *gorm.DB
	cfg         *config.Config
	syncService *service.SyncService
	startTime   time.Time
}

func NewAdminHandler(db *gorm.DB, cfg *config.Config, syncService *service.SyncService) *AdminHandler {
	return &AdminHandler{
		db:          db,
		cfg:         cfg,
		syncService: syncService,
		startTime:   time.Now(),
	}
}

// GetSystemStats computes system health, model status, and high-level KPIs
func (h *AdminHandler) GetSystemStats(w http.ResponseWriter, r *http.Request) {
	var userCount, mailboxCount, expenseCount int64
	var totalVolume float64

	h.db.Model(&models.User{}).Count(&userCount)
	h.db.Model(&models.EmailAccount{}).Count(&mailboxCount)
	h.db.Model(&models.Expense{}).Count(&expenseCount)
	h.db.Model(&models.Expense{}).Where("transaction_type != 'CREDIT'").Select("COALESCE(SUM(amount), 0)").Scan(&totalVolume)

	// Last 24 hours syncs stats
	yesterday := time.Now().Add(-24 * time.Hour)
	var syncsLast24h, successfulSyncsLast24h, paymentsFound24h int64
	h.db.Model(&models.SyncLog{}).Where("started_at >= ?", yesterday).Count(&syncsLast24h)
	h.db.Model(&models.SyncLog{}).Where("started_at >= ? AND status = 'SUCCESS'", yesterday).Count(&successfulSyncsLast24h)
	h.db.Model(&models.SyncLog{}).Where("started_at >= ?", yesterday).Select("COALESCE(SUM(payments_found), 0)").Scan(&paymentsFound24h)

	successRate := 100.0
	if syncsLast24h > 0 {
		successRate = math.Round((float64(successfulSyncsLast24h)/float64(syncsLast24h))*10000) / 100
	}

	// Check Ollama connection
	llmStatus := "CONNECTED"
	if h.cfg.LLMEnabled {
		client := &http.Client{Timeout: 2 * time.Second}
		parsedURL, err := url.Parse(h.cfg.OllamaBaseURL)
		if err == nil {
			parsedURL.Path = "/api/tags"
			resp, err := client.Get(parsedURL.String())
			if err != nil || resp.StatusCode != http.StatusOK {
				llmStatus = "FALLBACK_HEURISTICS"
			} else {
				resp.Body.Close()
			}
		} else {
			llmStatus = "FALLBACK_HEURISTICS"
		}
	} else {
		llmStatus = "DISABLED"
	}

	uptime := int64(time.Since(h.startTime).Seconds())

	stats := models.AdminSystemStats{
		TotalUsers:           userCount,
		TotalMailboxes:       mailboxCount,
		TotalExpenses:        expenseCount,
		TotalVolumeTracked:   math.Round(totalVolume*100) / 100,
		ActiveSyncWorkers:    h.cfg.SyncWorkers,
		DatabaseDriver:       h.cfg.DBDriver,
		LLMEngine:            "Ollama (Local)",
		LLMModel:             h.cfg.OllamaModel,
		LLMStatus:            llmStatus,
		NextScheduledCron:    h.cfg.CronSchedule,
		Last24hSyncsCount:    syncsLast24h,
		Last24hSuccessRate:   successRate,
		Last24hPaymentsFound: paymentsFound24h,
		ServerUptimeSeconds:  uptime,
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetUsersList returns all registered users with their account counts and totals
func (h *AdminHandler) GetUsersList(w http.ResponseWriter, r *http.Request) {
	var users []models.User
	if err := h.db.Preload("Accounts").Find(&users).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var adminUsers []models.AdminUserView
	for _, u := range users {
		var expCount int64
		var totalSpent float64
		h.db.Model(&models.Expense{}).Where("user_id = ?", u.ID).Count(&expCount)
		h.db.Model(&models.Expense{}).Where("user_id = ? AND transaction_type != 'CREDIT'", u.ID).Select("COALESCE(SUM(amount), 0)").Scan(&totalSpent)

		adminUsers = append(adminUsers, models.AdminUserView{
			ID:             u.ID,
			Email:          u.Email,
			FullName:       u.FullName,
			MailboxesCount: len(u.Accounts),
			ExpensesCount:  expCount,
			TotalSpent:     math.Round(totalSpent*100) / 100,
			Accounts:       u.Accounts,
			CreatedAt:      u.CreatedAt,
		})
	}

	respondJSON(w, http.StatusOK, adminUsers)
}

// GetGlobalSyncLogs returns the last 50 sync runs across all users
func (h *AdminHandler) GetGlobalSyncLogs(w http.ResponseWriter, r *http.Request) {
	var logs []models.SyncLog
	if err := h.db.Order("started_at DESC").Limit(50).Find(&logs).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, logs)
}

// GetGlobalTransactions returns the last 50 transactions extracted across the entire system
func (h *AdminHandler) GetGlobalTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var expenses []models.Expense
	if err := h.db.Order("date DESC").Limit(limit).Find(&expenses).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, expenses)
}

// TriggerGlobalSync triggers parallel sync across all active mailboxes on-demand
func (h *AdminHandler) TriggerGlobalSync(w http.ResponseWriter, r *http.Request) {
	if h.syncService.IsSyncing() {
		respondJSON(w, http.StatusConflict, map[string]string{
			"message": "sync is already in progress",
			"status":  "RUNNING",
		})
		return
	}

	go func() {
		_ = h.syncService.SyncAllActiveAccounts(context.Background(), "ADMIN_MANUAL")
	}()

	respondJSON(w, http.StatusAccepted, map[string]string{
		"message": "Global multi-user sync triggered in background",
		"status":  "ACCEPTED",
	})
}
