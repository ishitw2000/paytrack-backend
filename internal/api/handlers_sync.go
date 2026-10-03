package api

import (
	"context"
	"net/http"

	"github.com/paytrack/backend/internal/models"
	"github.com/paytrack/backend/internal/service"
	"gorm.io/gorm"
)

type SyncHandler struct {
	db          *gorm.DB
	syncService *service.SyncService
}

func NewSyncHandler(db *gorm.DB, syncService *service.SyncService) *SyncHandler {
	return &SyncHandler{
		db:          db,
		syncService: syncService,
	}
}

// TriggerManualSync handles on-demand sync for all connected mailboxes of the authenticated user
func (h *SyncHandler) TriggerManualSync(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	if h.syncService.IsSyncing() {
		respondJSON(w, http.StatusConflict, map[string]string{
			"message": "sync is already in progress",
			"status":  "RUNNING",
		})
		return
	}

	var accounts []models.EmailAccount
	if err := h.db.Where("user_id = ? AND is_active = ?", userID, true).Find(&accounts).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if len(accounts) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "no active email accounts connected. Please add an email account first at POST /api/v1/accounts",
		})
		return
	}

	isAsync := r.URL.Query().Get("async") == "true"
	if isAsync {
		go func() {
			for i := range accounts {
				_, _ = h.syncService.SyncUserAccount(context.Background(), &accounts[i], "MANUAL")
			}
		}()
		respondJSON(w, http.StatusAccepted, map[string]string{
			"message": "manual sync initiated in background for user accounts",
			"status":  "ACCEPTED",
		})
		return
	}

	var syncLogs []*models.SyncLog
	for i := range accounts {
		logRes, err := h.syncService.SyncUserAccount(r.Context(), &accounts[i], "MANUAL")
		if err != nil {
			syncLogs = append(syncLogs, logRes)
		} else {
			syncLogs = append(syncLogs, logRes)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "sync completed successfully",
		"results": syncLogs,
	})
}

// GetSyncStatus returns sync status for the authenticated user
func (h *SyncHandler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	isSyncing := h.syncService.IsSyncing()
	lastSync := h.syncService.LastSyncedAt()

	var lastLog models.SyncLog
	h.db.Where("user_id = ?", userID).Order("started_at DESC").First(&lastLog)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"is_syncing":     isSyncing,
		"last_synced_at": lastSync,
		"last_sync_log":  lastLog,
	})
}

// GetSyncLogs returns recent synchronization history for the authenticated user
func (h *SyncHandler) GetSyncLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var logs []models.SyncLog
	if err := h.db.Where("user_id = ?", userID).Order("started_at DESC").Limit(20).Find(&logs).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, logs)
}
