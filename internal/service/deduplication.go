package service

import (
	"math"
	"strings"
	"time"

	"github.com/paytrack/backend/internal/models"
	"gorm.io/gorm"
)

// DeduplicationEngine provides multi-tier transaction deduplication
// Layer 1: Exact Message-ID match (prevents re-ingesting already synced emails)
// Layer 2: Fuzzy sliding window match (same user, amount within precision, normalized merchant, within ±12 hours)
type DeduplicationEngine struct {
	db     *gorm.DB
	window time.Duration
}

// NewDeduplicationEngine creates a new deduplication engine with configurable time window
func NewDeduplicationEngine(db *gorm.DB, window time.Duration) *DeduplicationEngine {
	if window <= 0 {
		window = 12 * time.Hour
	}
	return &DeduplicationEngine{
		db:     db,
		window: window,
	}
}

// CheckMessageID returns true if an expense with the same Message-ID already exists for this user
func (d *DeduplicationEngine) CheckMessageID(userID uint, messageID string) (bool, error) {
	if messageID == "" {
		return false, nil
	}

	var existing models.Expense
	err := d.db.Where("user_id = ? AND message_id = ?", userID, messageID).First(&existing).Error
	if err == nil {
		return true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	return false, err
}

// CheckFuzzyDuplicate checks if a transaction with matching amount and merchant exists within the sliding time window
func (d *DeduplicationEngine) CheckFuzzyDuplicate(userID uint, amount float64, merchant string, txDate time.Time) (bool, *models.Expense, error) {
	if amount <= 0 || merchant == "" {
		return false, nil, nil
	}

	normalizedTarget := NormalizeMerchantName(merchant)
	startWindow := txDate.Add(-d.window)
	endWindow := txDate.Add(d.window)

	var candidates []models.Expense
	// Query by user and date window first for indexed speed
	err := d.db.Where("user_id = ? AND date BETWEEN ? AND ?", userID, startWindow, endWindow).Find(&candidates).Error
	if err != nil {
		return false, nil, err
	}

	for _, cand := range candidates {
		// Amount equality within 1 cent / 0.01 tolerance
		if math.Abs(cand.Amount-amount) < 0.01 {
			if NormalizeMerchantName(cand.Merchant) == normalizedTarget {
				return true, &cand, nil
			}
		}
	}

	return false, nil, nil
}

// IsDuplicate performs both exact and fuzzy deduplication checks in sequence
func (d *DeduplicationEngine) IsDuplicate(userID uint, messageID string, amount float64, merchant string, txDate time.Time) (bool, string, error) {
	// 1. Exact Message-ID check
	if messageID != "" {
		dupMsg, err := d.CheckMessageID(userID, messageID)
		if err != nil {
			return false, "", err
		}
		if dupMsg {
			return true, "message_id_match", nil
		}
	}

	// 2. Fuzzy window check
	dupFuzzy, _, err := d.CheckFuzzyDuplicate(userID, amount, merchant, txDate)
	if err != nil {
		return false, "", err
	}
	if dupFuzzy {
		return true, "fuzzy_time_window_match", nil
	}

	return false, "", nil
}

// NormalizeMerchantName cleans merchant strings for robust fuzzy comparison
func NormalizeMerchantName(name string) string {
	cleaned := strings.ToLower(strings.TrimSpace(name))
	// Strip common payment suffixes
	suffixes := []string{" upi", " pvt ltd", " ltd", " inc", " corp", " store", " in", ".com"}
	for _, sfx := range suffixes {
		cleaned = strings.TrimSuffix(cleaned, sfx)
	}
	return strings.TrimSpace(cleaned)
}
