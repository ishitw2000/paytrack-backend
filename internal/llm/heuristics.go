package llm

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/paytrack/backend/internal/merchant"
	"github.com/paytrack/backend/internal/models"
)

// HeuristicExtract provides high-speed regex parsing with dynamic merchant dictionary matching
func HeuristicExtract(subject, sender, body, defaultCurrency string) models.ExtractedPayment {
	fullText := subject + " " + sender + " " + body

	amount, currency := extractAmountAndCurrency(fullText, subject, sender, defaultCurrency)
	if amount <= 0 {
		return models.ExtractedPayment{IsPayment: false}
	}

	merchantName, category := extractMerchantAndCategory(subject, body, sender)
	paymentMethod := extractPaymentMethod(fullText)
	txnType := extractTransactionType(fullText)

	// If a valid merchant was detected, register it dynamically in the registry
	if merchantName != "" && merchantName != "Unknown Merchant" {
		if merchant.GlobalRegistry != nil {
			merchant.GlobalRegistry.LearnMerchant(merchantName, category)
		}
	}

	symbol := currency
	if currency == "INR" {
		symbol = "₹"
	} else if currency == "USD" {
		symbol = "$"
	} else if currency == "EUR" {
		symbol = "€"
	} else if currency == "GBP" {
		symbol = "£"
	}

	summaryText := "Transaction of " + symbol + strconv.FormatFloat(amount, 'f', 2, 64)
	if merchantName != "" && merchantName != "Unknown Merchant" {
		summaryText += " with " + merchantName
	}
	if paymentMethod != "" {
		summaryText += " via " + paymentMethod
	}
	if category != "" && category != "General Expense" {
		summaryText += " (" + category + ")"
	}
	if subject != "" {
		summaryText += " — " + subject
	}

	return models.ExtractedPayment{
		IsPayment:       true,
		Amount:          amount,
		Currency:        currency,
		Merchant:        merchantName,
		Category:        category,
		PaymentMethod:   paymentMethod,
		TransactionType: txnType,
		ConfidenceScore: 0.85,
		Summary:         summaryText,
	}
}

func extractAmountAndCurrency(text, subject, sender, defaultCurrency string) (float64, string) {
	if defaultCurrency == "" {
		defaultCurrency = "INR"
	}

	// 1. Explicit INR / Rupees matchers (highest priority for Indian banking/UPI alerts)
	inrPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:rs\.?|inr|₹)\s*([0-9]+(?:,[0-9]{2,3})*(?:\.[0-9]{1,2})?)`),
		regexp.MustCompile(`(?i)([0-9]+(?:,[0-9]{2,3})*(?:\.[0-9]{1,2})?)\s*(?:rs\.?|inr|₹|\/-)`),
		regexp.MustCompile(`(?i)(?:debited|spent|paid|charged|amount of|sent|received|total)\s*(?:by|of)?\s*(?:rs\.?|inr|₹)\s*([0-9]+(?:,[0-9]{2,3})*(?:\.[0-9]{1,2})?)`),
	}

	for _, p := range inrPatterns {
		matches := p.FindStringSubmatch(text)
		if len(matches) > 1 {
			if val := parseAmount(matches[1]); val > 0 {
				return val, "INR"
			}
		}
	}

	// 2. USD / Dollars matchers
	usdPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:\$|usd)\s*([0-9]+(?:,[0-9]{3})*(?:\.[0-9]{1,2})?)`),
		regexp.MustCompile(`(?i)([0-9]+(?:,[0-9]{3})*(?:\.[0-9]{1,2})?)\s*(?:usd|\$)`),
	}
	for _, p := range usdPatterns {
		matches := p.FindStringSubmatch(text)
		if len(matches) > 1 {
			if val := parseAmount(matches[1]); val > 0 {
				return val, "USD"
			}
		}
	}

	// 3. EUR / Euro matchers
	eurPattern := regexp.MustCompile(`(?i)(?:€|eur)\s*([0-9]+(?:,[0-9]{3})*(?:\.[0-9]{1,2})?)`)
	if matches := eurPattern.FindStringSubmatch(text); len(matches) > 1 {
		if val := parseAmount(matches[1]); val > 0 {
			return val, "EUR"
		}
	}

	// 4. GBP / British Pound matchers
	gbpPattern := regexp.MustCompile(`(?i)(?:£|gbp)\s*([0-9]+(?:,[0-9]{3})*(?:\.[0-9]{1,2})?)`)
	if matches := gbpPattern.FindStringSubmatch(text); len(matches) > 1 {
		if val := parseAmount(matches[1]); val > 0 {
			return val, "GBP"
		}
	}

	// 5. Generic debit/spent amounts (check sender domain to determine currency)
	genericPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:debited|spent|paid|charged|amount of|total:?|paid to)\s*(?:by|for)?\s*([0-9]+(?:,[0-9]{2,3})*(?:\.[0-9]{1,2})?)`),
	}
	for _, p := range genericPatterns {
		matches := p.FindStringSubmatch(text)
		if len(matches) > 1 {
			if val := parseAmount(matches[1]); val > 0 {
				// Check if sender or text mentions Indian banks/apps
				detectedCurrency := defaultCurrency
				lowerSender := strings.ToLower(sender + " " + subject)
				if isIndianFinancialEntity(lowerSender) {
					detectedCurrency = "INR"
				}
				return val, detectedCurrency
			}
		}
	}

	return 0, defaultCurrency
}

func parseAmount(numStr string) float64 {
	cleanNum := strings.ReplaceAll(numStr, ",", "")
	cleanNum = strings.TrimSuffix(cleanNum, "/-")
	cleanNum = strings.TrimSpace(cleanNum)
	val, err := strconv.ParseFloat(cleanNum, 64)
	if err == nil && val > 0 {
		return val
	}
	return 0
}

func extractMerchantAndCategory(subject, body, sender string) (string, string) {
	// 1. Check known merchant dictionary in global registry first!
	if merchant.GlobalRegistry != nil {
		if name, cat, found := merchant.GlobalRegistry.FindMatchingMerchant(subject, sender, body); found {
			return name, cat
		}
	}

	// 2. Pattern matching for "paid to [Merchant]", "at [Merchant]", "order from [Merchant]", "VPA [Merchant]"
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:paid to|transferred to|info\/|spent at|order at|at|to)\s+([A-Za-z0-9\.\-_& ]{2,30}?)(?:\s+(?:on|for|ref|using|via|dated|\.|\n|\r))`),
		regexp.MustCompile(`(?i)(?:order from|receipt from|invoice from|bill from)\s+([A-Za-z0-9\.\-_& ]{2,30})`),
		regexp.MustCompile(`(?i)vpa\s+([A-Za-z0-9\.\-_]{2,30})@`),
	}

	for _, re := range patterns {
		matches := re.FindStringSubmatch(subject + " \n " + body)
		if len(matches) > 1 {
			foundMerchant := strings.TrimSpace(matches[1])
			if isValidMerchantName(foundMerchant) {
				return cleanMerchantName(foundMerchant), categorizeMerchant(foundMerchant, subject+" "+body)
			}
		}
	}

	// 3. Fallback: Parse from sender name / domain
	cleanSender := sender
	if idx := strings.Index(sender, "<"); idx != -1 {
		cleanSender = strings.TrimSpace(sender[:idx])
	}
	cleanSender = strings.Trim(cleanSender, "\"")
	if cleanSender != "" && isValidMerchantName(cleanSender) {
		return cleanMerchantName(cleanSender), categorizeMerchant(cleanSender, subject+" "+body)
	}

	return "Unknown Merchant", "General Expense"
}

func isValidMerchantName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if len(n) < 2 || len(n) > 40 {
		return false
	}
	// Filter out common false-positive words
	blacklist := []string{"your account", "bank", "alert", "notification", "transaction", "payment", "card", "statement", "update", "service", "customer", "support", "no-reply", "noreply", "info", "mailer"}
	for _, b := range blacklist {
		if n == b {
			return false
		}
	}
	return true
}

func cleanMerchantName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "\"'.,")
	return strings.Title(name)
}

func isIndianFinancialEntity(text string) bool {
	keywords := []string{
		"hdfc", "icici", "sbi", "axis", "kotak", "paytm", "phonepe",
		"gpay", "cred", "razorpay", "pnb", "bob", "canara", "yesbank",
		"indusind", "idfc", "swiggy", "zomato", "myntra", "flipkart",
		"blinkit", "zepto", "upi", "vpa", "neft", "imps", "inr", "rs.", "rs ", "₹",
	}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func categorizeMerchant(merchant, text string) string {
	m := strings.ToLower(merchant + " " + text)

	switch {
	case strings.Contains(m, "swiggy") || strings.Contains(m, "zomato") || strings.Contains(m, "uber eats") ||
		strings.Contains(m, "doordash") || strings.Contains(m, "starbucks") || strings.Contains(m, "mcdonald") ||
		strings.Contains(m, "dominos") || strings.Contains(m, "pizza") || strings.Contains(m, "burger") ||
		strings.Contains(m, "restaurant") || strings.Contains(m, "cafe") || strings.Contains(m, "food") || strings.Contains(m, "dine"):
		return "Food & Dining"

	case strings.Contains(m, "blinkit") || strings.Contains(m, "zepto") || strings.Contains(m, "instamart") ||
		strings.Contains(m, "bigbasket") || strings.Contains(m, "dunzo") || strings.Contains(m, "dmart") ||
		strings.Contains(m, "grocery") || strings.Contains(m, "groceries") || strings.Contains(m, "supermarket"):
		return "Groceries & Essentials"

	case strings.Contains(m, "myntra") || strings.Contains(m, "amazon") || strings.Contains(m, "flipkart") ||
		strings.Contains(m, "ajio") || strings.Contains(m, "meesho") || strings.Contains(m, "nykaa") ||
		strings.Contains(m, "zara") || strings.Contains(m, "h&m") || strings.Contains(m, "shopping") || strings.Contains(m, "retail"):
		return "Shopping"

	case strings.Contains(m, "uber") || strings.Contains(m, "ola") || strings.Contains(m, "rapido") ||
		strings.Contains(m, "gas") || strings.Contains(m, "petrol") || strings.Contains(m, "fuel") ||
		strings.Contains(m, "flight") || strings.Contains(m, "irctc") || strings.Contains(m, "makemytrip") ||
		strings.Contains(m, "indigo") || strings.Contains(m, "air india") || strings.Contains(m, "transit"):
		return "Transportation"

	case strings.Contains(m, "netflix") || strings.Contains(m, "spotify") || strings.Contains(m, "youtube") ||
		strings.Contains(m, "hotstar") || strings.Contains(m, "bookmyshow") || strings.Contains(m, "pvr") ||
		strings.Contains(m, "prime video") || strings.Contains(m, "apple.com/bill") || strings.Contains(m, "google play"):
		return "Entertainment & Subscriptions"

	case strings.Contains(m, "jio") || strings.Contains(m, "airtel") || strings.Contains(m, "vi") ||
		strings.Contains(m, "electricity") || strings.Contains(m, "broadband") || strings.Contains(m, "wifi") ||
		strings.Contains(m, "utility") || strings.Contains(m, "recharge") || strings.Contains(m, "gas bill"):
		return "Utilities & Bills"

	case strings.Contains(m, "pharmacy") || strings.Contains(m, "1mg") || strings.Contains(m, "pharmeasy") ||
		strings.Contains(m, "apollo") || strings.Contains(m, "medical") || strings.Contains(m, "cult.fit") || strings.Contains(m, "hospital"):
		return "Healthcare"

	case strings.Contains(m, "github") || strings.Contains(m, "openai") || strings.Contains(m, "chatgpt") ||
		strings.Contains(m, "aws") || strings.Contains(m, "google cloud") || strings.Contains(m, "hosting") || strings.Contains(m, "software"):
		return "Software & Tech"

	case strings.Contains(m, "salary") || strings.Contains(m, "interest") || strings.Contains(m, "dividend") ||
		strings.Contains(m, "refund") || strings.Contains(m, "cashback"):
		return "Income & Refunds"

	default:
		return "General Expense"
	}
}

func extractPaymentMethod(text string) string {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "upi") || strings.Contains(t, "vpa") || strings.Contains(t, "gpay") || strings.Contains(t, "phonepe") || strings.Contains(t, "paytm"):
		return "UPI"
	case strings.Contains(t, "credit card") || strings.Contains(t, "card ending"):
		return "Credit Card"
	case strings.Contains(t, "debit card"):
		return "Debit Card"
	case strings.Contains(t, "net banking") || strings.Contains(t, "neft") || strings.Contains(t, "imps") || strings.Contains(t, "rtgs") || strings.Contains(t, "ach"):
		return "Net Banking"
	case strings.Contains(t, "wallet") || strings.Contains(t, "apple pay") || strings.Contains(t, "google pay"):
		return "Digital Wallet"
	case strings.Contains(t, "paypal"):
		return "PayPal"
	default:
		return "Bank / Card"
	}
}

func extractTransactionType(text string) string {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "credited") || strings.Contains(t, "received") || strings.Contains(t, "refund"):
		return "CREDIT"
	case strings.Contains(t, "subscription") || strings.Contains(t, "autopay") || strings.Contains(t, "recurring"):
		return "SUBSCRIPTION"
	case strings.Contains(t, "invoice"):
		return "INVOICE"
	default:
		return "DEBIT"
	}
}
