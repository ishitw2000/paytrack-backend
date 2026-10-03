package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/paytrack/backend/internal/config"
	"github.com/paytrack/backend/internal/models"
)

type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

type ollamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Format  string                 `json:"format"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

func NewOllamaClient(cfg *config.Config) *OllamaClient {
	timeout := time.Duration(cfg.LLMTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &OllamaClient{
		baseURL: cfg.OllamaBaseURL,
		model:   cfg.OllamaModel,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// AnalyzeEmail sends the email to the local Ollama LLM with candidate merchant hints and strict currency guidance
func (oc *OllamaClient) AnalyzeEmail(ctx context.Context, subject, sender, body, candidateMerchant, candidateCategory, defaultCurrency string) (*models.ExtractedPayment, error) {
	merchantHint := ""
	if candidateMerchant != "" {
		merchantHint = fmt.Sprintf("\nHINT: Pre-filter detected candidate merchant: \"%s\" (Category: %s). Use this if confirmed by email.", candidateMerchant, candidateCategory)
	}

	if defaultCurrency == "" {
		defaultCurrency = "INR"
	}

	prompt := fmt.Sprintf(`You are a financial email receipt and transaction parser.
Analyze this email and determine if it represents a financial payment, transaction, bank debit/credit alert, order receipt, or invoice.

EMAIL SUBJECT: %s
EMAIL SENDER: %s
EMAIL CONTENT:
%s
%s

EXTRACTION INSTRUCTIONS:
1. CURRENCY: Look carefully for currency symbols (₹, Rs, Rs., INR, $, USD, €, EUR). If the email is from an Indian service/bank (e.g. Swiggy, Zomato, Myntra, Flipkart, HDFC, ICICI, SBI, UPI), the currency is "INR". If not explicitly stated, use "%s".
2. AMOUNT: Extract the exact numerical price without currency symbols or commas (e.g. 1450.00).
3. MERCHANT: The exact business/store name (e.g. "Swiggy", "Zomato", "Myntra", "Amazon", "Uber", "Netflix", "Starbucks"). Do not use generic words like "Bank Alert" or "Payment".
4. CATEGORY: One of: "Food & Dining", "Shopping", "Groceries & Essentials", "Transportation", "Entertainment & Subscriptions", "Utilities & Bills", "Healthcare", "Software & Tech", "Income & Refunds", "General Expense".
5. PAYMENT_METHOD: e.g. "UPI", "Credit Card", "Debit Card", "Net Banking", "Digital Wallet", "Bank Transfer".
6. SUMMARY: Write a concise, informative 1-2 sentence human summary explaining WHAT was bought, WHY and HOW the payment occurred (e.g. "Paid ₹1,299 for clothing order on Myntra using HDFC Credit Card", "Swiggy food order delivered and paid via UPI", "Monthly Netflix subscription charged to Visa card ending in 4120").

Return ONLY a valid JSON object matching this schema:
{
  "is_payment": boolean,
  "amount": number,
  "currency": string,
  "merchant": string,
  "category": string,
  "payment_method": string,
  "transaction_type": string,
  "date": string,
  "confidence_score": number,
  "summary": string
}
`, subject, sender, body, merchantHint, defaultCurrency)

	reqBody := ollamaGenerateRequest{
		Model:  oc.model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
		Options: map[string]interface{}{
			"temperature": 0.1,
		},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ollama request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", oc.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := oc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama http call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned non-200 status: %d", resp.StatusCode)
	}

	var ollamaResp ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode ollama response: %w", err)
	}

	var extracted models.ExtractedPayment
	if err := json.Unmarshal([]byte(ollamaResp.Response), &extracted); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LLM JSON output: %w, raw response: %s", err, ollamaResp.Response)
	}

	// Sanity checks on currency & merchant
	if extracted.Currency == "" {
		extracted.Currency = defaultCurrency
	}
	if candidateMerchant != "" && (extracted.Merchant == "" || strings.EqualFold(extracted.Merchant, "Unknown") || strings.EqualFold(extracted.Merchant, "Unknown Merchant")) {
		extracted.Merchant = candidateMerchant
		if extracted.Category == "" {
			extracted.Category = candidateCategory
		}
	}

	return &extracted, nil
}
