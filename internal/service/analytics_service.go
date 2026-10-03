package service

import (
	"fmt"
	"math"
	"time"

	"github.com/paytrack/backend/internal/models"
	"gorm.io/gorm"
)

type AnalyticsService struct {
	db          *gorm.DB
	syncService *SyncService
}

func NewAnalyticsService(db *gorm.DB, syncService *SyncService) *AnalyticsService {
	return &AnalyticsService{
		db:          db,
		syncService: syncService,
	}
}

// GetMonthlyAnalytics computes detailed analytics for a given user, year, and month
func (a *AnalyticsService) GetMonthlyAnalytics(userID uint, year, month int) (*models.MonthlyAnalytics, error) {
	if year == 0 || month == 0 {
		now := time.Now()
		year = now.Year()
		month = int(now.Month())
	}

	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0)

	var expenses []models.Expense
	err := a.db.Where("user_id = ? AND date >= ? AND date < ?", userID, startDate, endDate).Order("date DESC").Find(&expenses).Error
	if err != nil {
		return nil, err
	}

	var totalDebit, totalCredit float64
	var txCount int64
	categoryMap := make(map[string]*models.CategorySpending)
	merchantMap := make(map[string]*models.MerchantSpending)
	dailyMap := make(map[string]*models.DailyTrend)

	numDays := endDate.Add(-time.Nanosecond).Day()
	for d := 1; d <= numDays; d++ {
		dayStr := fmt.Sprintf("%04d-%02d-%02d", year, month, d)
		dailyMap[dayStr] = &models.DailyTrend{Date: dayStr, Total: 0, Count: 0}
	}

	for _, exp := range expenses {
		txCount++
		if exp.TransactionType == "CREDIT" {
			totalCredit += exp.Amount
		} else {
			totalDebit += exp.Amount

			catName := exp.Category
			if catName == "" {
				catName = "Uncategorized"
			}
			if _, exists := categoryMap[catName]; !exists {
				categoryMap[catName] = &models.CategorySpending{Category: catName, Total: 0, Count: 0}
			}
			categoryMap[catName].Total += exp.Amount
			categoryMap[catName].Count++

			merchName := exp.Merchant
			if merchName == "" {
				merchName = "Unknown"
			}
			if _, exists := merchantMap[merchName]; !exists {
				merchantMap[merchName] = &models.MerchantSpending{Merchant: merchName, Total: 0, Count: 0}
			}
			merchantMap[merchName].Total += exp.Amount
			merchantMap[merchName].Count++

			dayStr := exp.Date.Format("2006-01-02")
			if item, exists := dailyMap[dayStr]; exists {
				item.Total += exp.Amount
				item.Count++
			}
		}
	}

	var categoryBreakdown []models.CategorySpending
	for _, cat := range categoryMap {
		if totalDebit > 0 {
			cat.Percentage = math.Round((cat.Total/totalDebit)*10000) / 100
		}
		categoryBreakdown = append(categoryBreakdown, *cat)
	}

	var topMerchants []models.MerchantSpending
	for _, m := range merchantMap {
		topMerchants = append(topMerchants, *m)
	}

	var dailyTrends []models.DailyTrend
	for d := 1; d <= numDays; d++ {
		dayStr := fmt.Sprintf("%04d-%02d-%02d", year, month, d)
		if item, exists := dailyMap[dayStr]; exists {
			dailyTrends = append(dailyTrends, *item)
		}
	}

	avgTx := 0.0
	if txCount > 0 {
		avgTx = math.Round((totalDebit/float64(txCount))*100) / 100
	}

	return &models.MonthlyAnalytics{
		Year:               year,
		Month:              month,
		MonthName:          startDate.Month().String(),
		TotalDebit:         math.Round(totalDebit*100) / 100,
		TotalCredit:        math.Round(totalCredit*100) / 100,
		NetExpense:         math.Round((totalDebit-totalCredit)*100) / 100,
		Currency:           "USD",
		TransactionCount:   txCount,
		AverageTransaction: avgTx,
		CategoryBreakdown:  categoryBreakdown,
		TopMerchants:       topMerchants,
		DailyTrends:        dailyTrends,
	}, nil
}

// GetDashboardSummary returns user-scoped summary for mobile home screen
func (a *AnalyticsService) GetDashboardSummary(userID uint) (*models.DashboardSummary, error) {
	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	prevMonthStart := currentMonthStart.AddDate(0, -1, 0)

	var currentMonthTotal, prevMonthTotal float64

	a.db.Model(&models.Expense{}).
		Where("user_id = ? AND date >= ? AND transaction_type != 'CREDIT'", userID, currentMonthStart).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&currentMonthTotal)

	a.db.Model(&models.Expense{}).
		Where("user_id = ? AND date >= ? AND date < ? AND transaction_type != 'CREDIT'", userID, prevMonthStart, currentMonthStart).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&prevMonthTotal)

	var totalTransactions int64
	a.db.Model(&models.Expense{}).Where("user_id = ?", userID).Count(&totalTransactions)

	momChange := 0.0
	if prevMonthTotal > 0 {
		momChange = math.Round(((currentMonthTotal-prevMonthTotal)/prevMonthTotal)*10000) / 100
	}

	var recentExpenses []models.Expense
	a.db.Where("user_id = ?", userID).Order("date DESC").Limit(10).Find(&recentExpenses)

	monthly, _ := a.GetMonthlyAnalytics(userID, now.Year(), int(now.Month()))
	var topCategories []models.CategorySpending
	if monthly != nil {
		topCategories = monthly.CategoryBreakdown
	}

	return &models.DashboardSummary{
		CurrentMonthExpense:  math.Round(currentMonthTotal*100) / 100,
		PreviousMonthExpense: math.Round(prevMonthTotal*100) / 100,
		MonthOverMonthChange: momChange,
		TotalTransactions:    totalTransactions,
		RecentExpenses:       recentExpenses,
		TopCategories:        topCategories,
		LastSyncedAt:         a.syncService.LastSyncedAt(),
	}, nil
}
