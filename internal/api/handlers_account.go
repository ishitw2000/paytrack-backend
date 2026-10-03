package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/paytrack/backend/internal/config"
	"github.com/paytrack/backend/internal/crypto"
	"github.com/paytrack/backend/internal/models"
	"github.com/paytrack/backend/internal/service"
	"gorm.io/gorm"
)

type AccountHandler struct {
	db          *gorm.DB
	cfg         *config.Config
	syncService *service.SyncService
}

func NewAccountHandler(db *gorm.DB, cfg *config.Config, syncService *service.SyncService) *AccountHandler {
	return &AccountHandler{db: db, cfg: cfg, syncService: syncService}
}

type AddAccountRequest struct {
	Provider     string `json:"provider"`      // "Gmail", "Outlook", "Yahoo", "iCloud", "Custom"
	EmailAddress string `json:"email_address"` // e.g. john@gmail.com
	AppPassword  string `json:"app_password"`  // App Password from email provider
	IMAPServer   string `json:"imap_server"`   // e.g. imap.gmail.com:993
	IMAPFolder   string `json:"imap_folder"`   // e.g. INBOX
}

// AddAccount connects a new email account for the authenticated user
func (h *AccountHandler) AddAccount(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req AddAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.EmailAddress = strings.TrimSpace(req.EmailAddress)
	if req.EmailAddress == "" || req.AppPassword == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "email_address and app_password are required"})
		return
	}

	if req.Provider == "" {
		req.Provider = "Gmail"
	}
	if req.IMAPServer == "" {
		switch strings.ToLower(req.Provider) {
		case "gmail":
			req.IMAPServer = "imap.gmail.com:993"
		case "outlook", "hotmail":
			req.IMAPServer = "outlook.office365.com:993"
		case "yahoo":
			req.IMAPServer = "imap.mail.yahoo.com:993"
		case "icloud":
			req.IMAPServer = "imap.mail.me.com:993"
		default:
			req.IMAPServer = "imap.gmail.com:993"
		}
	}
	if req.IMAPFolder == "" {
		req.IMAPFolder = "INBOX"
	}

	// Encrypt App Password before saving to database
	encryptedPw, err := crypto.Encrypt(req.AppPassword, h.cfg.EncryptionKey)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to secure password"})
		return
	}

	account := models.EmailAccount{
		UserID:            userID,
		Provider:          req.Provider,
		EmailAddress:      req.EmailAddress,
		IMAPServer:        req.IMAPServer,
		IMAPFolder:        req.IMAPFolder,
		EncryptedPassword: encryptedPw,
		IsActive:          true,
	}

	if err := h.db.Create(&account).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save account"})
		return
	}

	respondJSON(w, http.StatusCreated, account)
}

// ListAccounts lists all connected email inboxes for the authenticated user
func (h *AccountHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var accounts []models.EmailAccount
	if err := h.db.Where("user_id = ?", userID).Find(&accounts).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, accounts)
}

// DeleteAccount removes a connected email account
func (h *AccountHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account ID"})
		return
	}

	if err := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.EmailAccount{}).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "account disconnected successfully"})
}
