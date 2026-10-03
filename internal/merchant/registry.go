package merchant

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// DefaultKnownMerchants is a comprehensive starter list of Indian and global merchants categorized
var DefaultKnownMerchants = map[string]string{
	// Food & Dining
	"Swiggy":             "Food & Dining",
	"Zomato":             "Food & Dining",
	"McDonalds":          "Food & Dining",
	"Dominos":            "Food & Dining",
	"Pizza Hut":          "Food & Dining",
	"Starbucks":          "Food & Dining",
	"KFC":                "Food & Dining",
	"Burger King":        "Food & Dining",
	"EatSure":            "Food & Dining",
	"Subway":             "Food & Dining",
	"Chai Point":         "Food & Dining",
	"Chaayos":            "Food & Dining",
	"Uber Eats":          "Food & Dining",
	"DoorDash":           "Food & Dining",

	// Quick Commerce & Groceries
	"Blinkit":            "Groceries & Essentials",
	"Zepto":              "Groceries & Essentials",
	"Instamart":          "Groceries & Essentials",
	"BigBasket":          "Groceries & Essentials",
	"Dunzo":              "Groceries & Essentials",
	"JioMart":            "Groceries & Essentials",
	"DMart":              "Groceries & Essentials",
	"Nature's Basket":    "Groceries & Essentials",

	// Shopping & E-Commerce
	"Myntra":             "Shopping",
	"Amazon":             "Shopping",
	"Flipkart":           "Shopping",
	"Ajio":               "Shopping",
	"Tata CLiQ":          "Shopping",
	"Nykaa":              "Shopping",
	"Meesho":             "Shopping",
	"Lenskart":           "Shopping",
	"Zara":               "Shopping",
	"H&M":                "Shopping",
	"Uniqlo":             "Shopping",
	"Decathlon":          "Shopping",
	"Croma":              "Shopping",
	"Reliance Digital":   "Shopping",
	"Apple":              "Shopping",
	"IKEA":               "Shopping",

	// Travel & Commute
	"Uber":               "Transportation",
	"Ola":                "Transportation",
	"Rapido":             "Transportation",
	"MakeMyTrip":         "Travel & Transport",
	"Cleartrip":          "Travel & Transport",
	"Goibibo":            "Travel & Transport",
	"Yatra":              "Travel & Transport",
	"IRCTC":              "Travel & Transport",
	"EaseMyTrip":         "Travel & Transport",
	"IndiGo":             "Travel & Transport",
	"Air India":          "Travel & Transport",
	"Vistara":            "Travel & Transport",
	"Akasa Air":          "Travel & Transport",
	"RedBus":             "Travel & Transport",
	"AbhiBus":            "Travel & Transport",
	"HPCL":               "Fuel & Gas",
	"BPCL":               "Fuel & Gas",
	"IOCL":               "Fuel & Gas",

	// Entertainment & Subscriptions
	"Netflix":            "Entertainment & Subscriptions",
	"Spotify":            "Entertainment & Subscriptions",
	"Amazon Prime":       "Entertainment & Subscriptions",
	"YouTube Premium":    "Entertainment & Subscriptions",
	"Disney+ Hotstar":    "Entertainment & Subscriptions",
	"BookMyShow":         "Entertainment & Subscriptions",
	"PVR Cinemas":        "Entertainment & Subscriptions",
	"INOX":               "Entertainment & Subscriptions",
	"Sony LIV":           "Entertainment & Subscriptions",
	"Zee5":               "Entertainment & Subscriptions",
	"Apple Services":     "Entertainment & Subscriptions",
	"Google Play":        "Entertainment & Subscriptions",

	// Utilities, Telecom & Recharges
	"Jio":                "Utilities & Telecom",
	"Airtel":             "Utilities & Telecom",
	"Vodafone Idea":      "Utilities & Telecom",
	"Vi":                 "Utilities & Telecom",
	"Tata Power":         "Utilities & Bills",
	"Adani Electricity":  "Utilities & Bills",
	"BESCOM":             "Utilities & Bills",
	"Mahanagar Gas":      "Utilities & Bills",
	"Indraprastha Gas":   "Utilities & Bills",
	"ACT Fibernet":       "Utilities & Bills",

	// Health & Fitness
	"Cult.fit":           "Health & Fitness",
	"1mg":                "Healthcare & Pharmacy",
	"PharmEasy":          "Healthcare & Pharmacy",
	"Apollo Pharmacy":    "Healthcare & Pharmacy",
	"Netmeds":            "Healthcare & Pharmacy",
	"Practo":             "Healthcare & Pharmacy",

	// Fintech & Payments
	"CRED":               "Credit Card Bill & Finance",
	"Paytm":              "Digital Wallet / UPI",
	"PhonePe":            "Digital Wallet / UPI",
	"Google Pay":         "Digital Wallet / UPI",
	"Razorpay":           "Payment Gateway",
	"Cashfree":           "Payment Gateway",
	"BillDesk":           "Payment Gateway",
	"PayU":               "Payment Gateway",

	// Software & Cloud
	"GitHub":             "Software & Cloud",
	"OpenAI":             "Software & Cloud",
	"ChatGPT":            "Software & Cloud",
	"Anthropic":          "Software & Cloud",
	"AWS":                "Software & Cloud",
	"Google Cloud":       "Software & Cloud",
	"DigitalOcean":       "Software & Cloud",
	"Vercel":             "Software & Cloud",
}

type Registry struct {
	filePath string
	merchants map[string]string // merchant name -> category
	mu        sync.RWMutex
}

var GlobalRegistry *Registry

func InitRegistry(dataDir string) *Registry {
	filePath := filepath.Join(dataDir, "merchants.txt")
	reg := &Registry{
		filePath:  filePath,
		merchants: make(map[string]string),
	}

	// Load defaults
	for m, c := range DefaultKnownMerchants {
		reg.merchants[strings.ToLower(m)] = c
	}

	// Load file overrides / additions if exists
	reg.loadFromFile()

	// Save back merged list
	reg.saveToFile()

	GlobalRegistry = reg
	log.Printf("[MERCHANT-REGISTRY] Initialized with %d registered merchants.\n", len(reg.merchants))
	return reg
}

// FindMatchingMerchant scans text for any known merchant from the registry
func (r *Registry) FindMatchingMerchant(subject, sender, body string) (name string, category string, found bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Prioritize Subject and Sender, then Body
	checkTexts := []string{
		subject,
		sender,
		body,
	}

	for _, text := range checkTexts {
		lowerText := strings.ToLower(text)
		for m, cat := range r.merchants {
			// Match exact word boundary to prevent false positives (e.g. "vi" in "service")
			pattern := `(?i)\b` + regexp.QuoteMeta(m) + `\b`
			matched, err := regexp.MatchString(pattern, lowerText)
			if err == nil && matched {
				return formatMerchantName(m), cat, true
			}
		}
	}

	return "", "", false
}

// LearnMerchant dynamically adds a newly discovered merchant to the registry and persists it
func (r *Registry) LearnMerchant(merchantName, category string) {
	merchantName = strings.TrimSpace(merchantName)
	if len(merchantName) < 2 || strings.EqualFold(merchantName, "Unknown Merchant") || strings.EqualFold(merchantName, "Unknown") {
		return
	}

	key := strings.ToLower(merchantName)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.merchants[key]; !exists {
		if category == "" {
			category = "General Expense"
		}
		r.merchants[key] = category
		log.Printf("[MERCHANT-REGISTRY] Learned new merchant: '%s' [%s]\n", merchantName, category)
		go r.saveToFile()
	}
}

func (r *Registry) loadFromFile() {
	file, err := os.Open(r.filePath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		merchantName := strings.TrimSpace(parts[0])
		category := "General Expense"
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			category = strings.TrimSpace(parts[1])
		}
		if merchantName != "" {
			r.merchants[strings.ToLower(merchantName)] = category
		}
	}
}

func (r *Registry) saveToFile() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_ = os.MkdirAll(filepath.Dir(r.filePath), 0755)
	file, err := os.Create(r.filePath)
	if err != nil {
		return
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	_, _ = writer.WriteString("# PayTrack Dynamic Merchant Registry\n")
	_, _ = writer.WriteString("# Format: MerchantName:Category\n\n")

	for m, c := range r.merchants {
		_, _ = writer.WriteString(formatMerchantName(m) + ":" + c + "\n")
	}
	_ = writer.Flush()
}

func formatMerchantName(s string) string {
	for original := range DefaultKnownMerchants {
		if strings.EqualFold(original, s) {
			return original
		}
	}
	return strings.Title(s)
}
