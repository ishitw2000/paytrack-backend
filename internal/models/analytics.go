package models

import "time"

// CategorySpending holds amount and percentage for a category
type CategorySpending struct {
	Category   string  `json:"category"`
	Total      float64 `json:"total"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// MerchantSpending holds amount and transaction count for top merchants
type MerchantSpending struct {
	Merchant string  `json:"merchant"`
	Total    float64 `json:"total"`
	Count    int64   `json:"count"`
}

// DailyTrend holds the spending for a specific day
type DailyTrend struct {
	Date  string  `json:"date"` // YYYY-MM-DD
	Total float64 `json:"total"`
	Count int64   `json:"count"`
}

// MonthlyAnalytics represents full breakdown of a given month
type MonthlyAnalytics struct {
	Year               int                `json:"year"`
	Month              int                `json:"month"`
	MonthName          string             `json:"month_name"`
	TotalDebit         float64            `json:"total_debit"`
	TotalCredit        float64            `json:"total_credit"`
	NetExpense         float64            `json:"net_expense"`
	Currency           string             `json:"currency"`
	TransactionCount   int64              `json:"transaction_count"`
	AverageTransaction float64            `json:"average_transaction"`
	CategoryBreakdown  []CategorySpending `json:"category_breakdown"`
	TopMerchants       []MerchantSpending `json:"top_merchants"`
	DailyTrends        []DailyTrend       `json:"daily_trends"`
}

// DashboardSummary is a high-level summary for the mobile home screen
type DashboardSummary struct {
	CurrentMonthExpense float64            `json:"current_month_expense"`
	PreviousMonthExpense float64           `json:"previous_month_expense"`
	MonthOverMonthChange float64           `json:"month_over_month_change_pct"`
	TotalTransactions   int64              `json:"total_transactions"`
	RecentExpenses      []Expense          `json:"recent_expenses"`
	TopCategories       []CategorySpending `json:"top_categories"`
	LastSyncedAt        *time.Time         `json:"last_synced_at,omitempty"`
}
