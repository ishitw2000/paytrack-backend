package models

import "time"

// AdminSystemStats provides high-level platform health and analytics
type AdminSystemStats struct {
	TotalUsers          int64   `json:"total_users"`
	TotalMailboxes      int64   `json:"total_mailboxes"`
	TotalExpenses       int64   `json:"total_expenses"`
	TotalVolumeTracked  float64 `json:"total_volume_tracked"`
	ActiveSyncWorkers   int     `json:"active_sync_workers"`
	DatabaseDriver      string  `json:"database_driver"`
	LLMEngine           string  `json:"llm_engine"`
	LLMModel            string  `json:"llm_model"`
	LLMStatus           string  `json:"llm_status"` // "CONNECTED", "FALLBACK_HEURISTICS"
	NextScheduledCron   string  `json:"next_scheduled_cron"`
	Last24hSyncsCount   int64   `json:"last_24h_syncs_count"`
	Last24hSuccessRate  float64 `json:"last_24h_success_rate"`
	Last24hPaymentsFound int64  `json:"last_24h_payments_found"`
	ServerUptimeSeconds int64   `json:"server_uptime_seconds"`
}

// AdminUserView represents a user row in the admin panel
type AdminUserView struct {
	ID             uint           `json:"id"`
	Email          string         `json:"email"`
	FullName       string         `json:"full_name"`
	MailboxesCount int            `json:"mailboxes_count"`
	ExpensesCount  int64          `json:"expenses_count"`
	TotalSpent     float64        `json:"total_spent"`
	Accounts       []EmailAccount `json:"accounts"`
	CreatedAt      time.Time      `json:"created_at"`
}
