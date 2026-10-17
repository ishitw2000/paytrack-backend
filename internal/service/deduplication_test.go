package service

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/paytrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}

	err = db.AutoMigrate(&models.Expense{})
	if err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}

	return db
}

func strPtr(s string) *string {
	return &s
}

func TestDeduplication_MessageID(t *testing.T) {
	db := setupTestDB(t)
	dedup := NewDeduplicationEngine(db, 12*time.Hour)

	userID := uint(1)
	msgID := "<txn-unique-12345@bank.com>"

	// Ensure no duplicate initially
	isDup, err := dedup.CheckMessageID(userID, msgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isDup {
		t.Errorf("expected isDup=false before insertion")
	}

	// Insert record
	exp := models.Expense{
		UserID:    userID,
		MessageID: strPtr(msgID),
		Amount:    150.00,
		Merchant:  "Uber",
		Date:      time.Now(),
	}
	if err := db.Create(&exp).Error; err != nil {
		t.Fatalf("failed to insert expense: %v", err)
	}

	// Now check duplicate for same user
	isDup, err = dedup.CheckMessageID(userID, msgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isDup {
		t.Errorf("expected isDup=true for existing message-id")
	}

	// Another user with same message-id (multi-tenant boundary check)
	isDupUser2, err := dedup.CheckMessageID(uint(2), msgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isDupUser2 {
		t.Errorf("expected isDup=false for different user (tenant isolation)")
	}
}

func TestDeduplication_FuzzySlidingWindow(t *testing.T) {
	db := setupTestDB(t)
	dedup := NewDeduplicationEngine(db, 12*time.Hour)

	userID := uint(42)
	baseTime := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)

	// Seed existing transaction: Swiggy INR 450.00 at 14:00
	existing := models.Expense{
		UserID:   userID,
		Amount:   450.00,
		Merchant: "Swiggy",
		Date:     baseTime,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("failed to insert test expense: %v", err)
	}

	tests := []struct {
		name        string
		targetUser  uint
		amount      float64
		merchant    string
		txDate      time.Time
		wantDup     bool
		description string
	}{
		{
			name:        "Exact fuzzy match within 1 hour",
			targetUser:  userID,
			amount:      450.00,
			merchant:    "Swiggy",
			txDate:      baseTime.Add(1 * time.Hour),
			wantDup:     true,
			description: "Same user, amount, merchant within 1h should be identified as duplicate",
		},
		{
			name:        "Normalized merchant match (Swiggy UPI vs Swiggy)",
			targetUser:  userID,
			amount:      450.00,
			merchant:    "Swiggy UPI",
			txDate:      baseTime.Add(-2 * time.Hour),
			wantDup:     true,
			description: "Merchant normalization should strip 'UPI' suffix and detect duplicate",
		},
		{
			name:        "Outside sliding window (+18 hours)",
			targetUser:  userID,
			amount:      450.00,
			merchant:    "Swiggy",
			txDate:      baseTime.Add(18 * time.Hour),
			wantDup:     false,
			description: "Transaction 18h later is outside 12h window and should not be blocked",
		},
		{
			name:        "Different amount at same merchant",
			targetUser:  userID,
			amount:      520.00,
			merchant:    "Swiggy",
			txDate:      baseTime.Add(30 * time.Minute),
			wantDup:     false,
			description: "Different amount should not be treated as duplicate",
		},
		{
			name:        "Different merchant with same amount",
			targetUser:  userID,
			amount:      450.00,
			merchant:    "Zomato",
			txDate:      baseTime.Add(10 * time.Minute),
			wantDup:     false,
			description: "Different merchant should not be treated as duplicate",
		},
		{
			name:        "Tenant Isolation (User 99 with same amount and merchant)",
			targetUser:  uint(99),
			amount:      450.00,
			merchant:    "Swiggy",
			txDate:      baseTime,
			wantDup:     false,
			description: "Different user should never be blocked by another user's transactions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isDup, _, err := dedup.CheckFuzzyDuplicate(tt.targetUser, tt.amount, tt.merchant, tt.txDate)
			if err != nil {
				t.Fatalf("CheckFuzzyDuplicate error: %v", err)
			}
			if isDup != tt.wantDup {
				t.Errorf("[%s] got isDup=%v, want %v (%s)", tt.name, isDup, tt.wantDup, tt.description)
			}
		})
	}
}

func TestDeduplication_IsDuplicateE2E(t *testing.T) {
	db := setupTestDB(t)
	dedup := NewDeduplicationEngine(db, 12*time.Hour)

	userID := uint(10)
	now := time.Now()

	// 1. Insert an existing transaction with a message ID
	msgID := "<order-9876@amazon.com>"
	existing := models.Expense{
		UserID:    userID,
		MessageID: &msgID,
		Amount:    89.99,
		Merchant:  "Amazon.com",
		Date:      now,
	}
	db.Create(&existing)

	// Test exact message ID match
	isDup, reason, err := dedup.IsDuplicate(userID, msgID, 89.99, "Amazon", now)
	if err != nil || !isDup || reason != "message_id_match" {
		t.Errorf("expected message_id_match, got isDup=%v, reason=%s, err=%v", isDup, reason, err)
	}

	// Test fuzzy match without message ID (e.g. from bank SMS/alert referencing same order)
	isDupFuzzy, reasonFuzzy, err := dedup.IsDuplicate(userID, "", 89.99, "Amazon", now.Add(2*time.Hour))
	if err != nil || !isDupFuzzy || reasonFuzzy != "fuzzy_time_window_match" {
		t.Errorf("expected fuzzy_time_window_match, got isDup=%v, reason=%s, err=%v", isDupFuzzy, reasonFuzzy, err)
	}

	// Test clean new transaction
	isDupClean, _, err := dedup.IsDuplicate(userID, "<new-msg@github.com>", 4.00, "GitHub", now)
	if err != nil || isDupClean {
		t.Errorf("expected non-duplicate, got isDup=%v, err=%v", isDupClean, err)
	}
}
