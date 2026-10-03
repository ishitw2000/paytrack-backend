package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/paytrack/backend/internal/models"
	"gorm.io/gorm"
)

type ExpenseHandler struct {
	db *gorm.DB
}

func NewExpenseHandler(db *gorm.DB) *ExpenseHandler {
	return &ExpenseHandler{db: db}
}

type PaginatedExpensesResponse struct {
	Data       []models.Expense `json:"data"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalCount int64            `json:"total_count"`
	TotalPages int              `json:"total_pages"`
}

// ListExpenses returns filtered and paginated list of expenses for the authenticated user
func (h *ExpenseHandler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	query := h.db.Model(&models.Expense{}).Where("user_id = ?", userID)

	if category := q.Get("category"); category != "" {
		query = query.Where("category = ?", category)
	}

	if txType := q.Get("type"); txType != "" {
		query = query.Where("transaction_type = ?", txType)
	}

	if search := q.Get("search"); search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where("(merchant LIKE ? OR raw_subject LIKE ? OR summary LIKE ? OR notes LIKE ?)",
			searchTerm, searchTerm, searchTerm, searchTerm)
	}

	if startDate := q.Get("start_date"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("date >= ?", t)
		}
	}
	if endDate := q.Get("end_date"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("date <= ?", t.Add(24*time.Hour))
		}
	}

	var totalCount int64
	query.Count(&totalCount)

	allowedSortFields := map[string]string{
		"date":       "date",
		"amount":     "amount",
		"merchant":   "merchant",
		"category":   "category",
		"created_at": "created_at",
		"id":         "id",
	}

	sortBy := q.Get("sort_by")
	order := q.Get("order")
	dbCol, okCol := allowedSortFields[sortBy]
	if !okCol {
		dbCol = "date"
	}
	if order != "asc" && order != "ASC" {
		order = "DESC"
	}
	query = query.Order(dbCol + " " + order)

	var expenses []models.Expense
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Find(&expenses).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	totalPages := int((totalCount + int64(limit) - 1) / int64(limit))

	respondJSON(w, http.StatusOK, PaginatedExpensesResponse{
		Data:       expenses,
		Page:       page,
		Limit:      limit,
		TotalCount: totalCount,
		TotalPages: totalPages,
	})
}

// GetExpenseByID returns a single expense by ID for the authenticated user
func (h *ExpenseHandler) GetExpenseByID(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expense ID"})
		return
	}

	var expense models.Expense
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&expense).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "expense not found"})
		return
	}

	respondJSON(w, http.StatusOK, expense)
}

// CreateManualExpense logs an expense manually for the authenticated user
func (h *ExpenseHandler) CreateManualExpense(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req models.Expense
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Amount <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "amount must be greater than 0"})
		return
	}

	req.UserID = userID
	if req.Date.IsZero() {
		req.Date = time.Now()
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	if req.Category == "" {
		req.Category = "General Expense"
	}
	if req.TransactionType == "" {
		req.TransactionType = "DEBIT"
	}
	req.IsManual = true
	req.ConfidenceScore = 1.0

	if err := h.db.Create(&req).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusCreated, req)
}

// UpdateExpense allows user to modify category, merchant, notes or amount
func (h *ExpenseHandler) UpdateExpense(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expense ID"})
		return
	}

	var existing models.Expense
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&existing).Error; err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "expense not found"})
		return
	}

	var updateData struct {
		Amount          *float64   `json:"amount"`
		Currency        *string    `json:"currency"`
		Merchant        *string    `json:"merchant"`
		Category        *string    `json:"category"`
		PaymentMethod   *string    `json:"payment_method"`
		TransactionType *string    `json:"transaction_type"`
		Notes           *string    `json:"notes"`
		Date            *time.Time `json:"date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid update body"})
		return
	}

	if updateData.Amount != nil {
		existing.Amount = *updateData.Amount
	}
	if updateData.Currency != nil {
		existing.Currency = *updateData.Currency
	}
	if updateData.Merchant != nil {
		existing.Merchant = *updateData.Merchant
	}
	if updateData.Category != nil {
		existing.Category = *updateData.Category
	}
	if updateData.PaymentMethod != nil {
		existing.PaymentMethod = *updateData.PaymentMethod
	}
	if updateData.TransactionType != nil {
		existing.TransactionType = *updateData.TransactionType
	}
	if updateData.Notes != nil {
		existing.Notes = *updateData.Notes
	}
	if updateData.Date != nil {
		existing.Date = *updateData.Date
	}

	if err := h.db.Save(&existing).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, existing)
}

// DeleteExpense removes an expense by ID
func (h *ExpenseHandler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expense ID"})
		return
	}

	if err := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Expense{}).Error; err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "expense deleted successfully"})
}

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}
