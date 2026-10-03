package llm

import (
	"context"
	"log"

	"github.com/paytrack/backend/internal/config"
	"github.com/paytrack/backend/internal/email"
	"github.com/paytrack/backend/internal/merchant"
	"github.com/paytrack/backend/internal/models"
)

type PaymentExtractor struct {
	cfg          *config.Config
	ollamaClient *OllamaClient
}

func NewPaymentExtractor(cfg *config.Config) *PaymentExtractor {
	return &PaymentExtractor{
		cfg:          cfg,
		ollamaClient: NewOllamaClient(cfg),
	}
}

// ExtractPaymentDetails pre-scans for known merchants, queries Ollama LLM, and falls back to heuristics
func (pe *PaymentExtractor) ExtractPaymentDetails(ctx context.Context, subject, sender, body string) models.ExtractedPayment {
	cleanBody := email.TruncateBody(body, 2000)

	// Pre-filter against merchant registry
	candidateMerchant, candidateCategory := "", ""
	if merchant.GlobalRegistry != nil {
		if mName, mCat, found := merchant.GlobalRegistry.FindMatchingMerchant(subject, sender, cleanBody); found {
			candidateMerchant = mName
			candidateCategory = mCat
		}
	}

	if pe.cfg.LLMEnabled {
		extracted, err := pe.ollamaClient.AnalyzeEmail(
			ctx,
			subject,
			sender,
			cleanBody,
			candidateMerchant,
			candidateCategory,
			pe.cfg.DefaultCurrency,
		)
		if err == nil && extracted != nil {
			log.Printf("[EXTRACTOR] LLM successfully analyzed email: %s -> IsPayment=%v, Amount=%.2f %s, Merchant='%s'\n",
				subject, extracted.IsPayment, extracted.Amount, extracted.Currency, extracted.Merchant)

			// Dynamically learn new merchant
			if extracted.IsPayment && extracted.Merchant != "" && merchant.GlobalRegistry != nil {
				merchant.GlobalRegistry.LearnMerchant(extracted.Merchant, extracted.Category)
			}

			return *extracted
		}
		log.Printf("[EXTRACTOR] LLM call failed or unavailable (%v), falling back to rule heuristics\n", err)
	}

	// Fallback to enhanced heuristic regex engine
	heuristicResult := HeuristicExtract(subject, sender, cleanBody, pe.cfg.DefaultCurrency)
	log.Printf("[EXTRACTOR] Heuristic analysis result for '%s': IsPayment=%v, Amount=%.2f %s, Merchant='%s'\n",
		subject, heuristicResult.IsPayment, heuristicResult.Amount, heuristicResult.Currency, heuristicResult.Merchant)
	return heuristicResult
}
