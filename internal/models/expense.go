package models

import (
	"time"
)

// Expense represents a financial transaction extracted from emails or manually created.
type Expense struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"index;not null" json:"user_id"`
	AccountID       uint      `gorm:"index" json:"account_id,omitempty"`
	EmailUID        uint32    `gorm:"index" json:"email_uid,omitempty"`
	MessageID       *string   `gorm:"index;size:255" json:"message_id,omitempty"`
	Sender          string    `gorm:"size:255" json:"sender"`
	RawSubject      string    `gorm:"size:500" json:"raw_subject"`
	Date            time.Time `gorm:"index" json:"date"`
	Amount          float64   `gorm:"type:decimal(12,2);index" json:"amount"`
	Currency        string    `gorm:"size:10;default:'USD'" json:"currency"`
	Merchant        string    `gorm:"size:255;index" json:"merchant"`
	Category        string    `gorm:"size:100;index;default:'Uncategorized'" json:"category"`
	PaymentMethod   string    `gorm:"size:100" json:"payment_method"` // e.g. "Credit Card", "Debit Card", "UPI", "Bank Transfer", "Wallet", "Cash"
	TransactionType string    `gorm:"size:50;default:'DEBIT'" json:"transaction_type"` // "DEBIT", "CREDIT", "SUBSCRIPTION", "INVOICE"
	ConfidenceScore float64   `gorm:"default:1.0" json:"confidence_score"` // 0.0 to 1.0 from LLM/heuristic
	Summary         string    `gorm:"type:text" json:"summary"`
	Notes           string    `gorm:"type:text" json:"notes,omitempty"`
	IsManual        bool      `gorm:"default:false" json:"is_manual"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SyncLog tracks every email sync execution
type SyncLog struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UserID           uint      `gorm:"index;not null" json:"user_id"`
	AccountID        uint      `gorm:"index" json:"account_id,omitempty"`
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      time.Time `json:"completed_at"`
	Status           string    `gorm:"size:50" json:"status"` // "SUCCESS", "PARTIAL", "FAILED", "RUNNING"
	TriggerType      string    `gorm:"size:50" json:"trigger_type"` // "CRON", "MANUAL"
	EmailsProcessed  int       `json:"emails_processed"`
	PaymentsFound    int       `json:"payments_found"`
	NewExpensesSaved int       `json:"new_expenses_saved"`
	ErrorMessage     string    `gorm:"type:text" json:"error_message,omitempty"`
}

// ExtractedPayment is the structured JSON output expected from LLM / parser
type ExtractedPayment struct {
	IsPayment       bool    `json:"is_payment"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	Merchant        string  `json:"merchant"`
	Category        string  `json:"category"`
	PaymentMethod   string  `json:"payment_method"`
	TransactionType string  `json:"transaction_type"` // "DEBIT", "CREDIT", "SUBSCRIPTION", "INVOICE"
	DateStr         string  `json:"date"`             // YYYY-MM-DD or RFC3339 if found in body
	ConfidenceScore float64 `json:"confidence_score"`
	Summary         string  `json:"summary"`
}
