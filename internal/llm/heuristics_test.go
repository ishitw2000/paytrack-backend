package llm

import (
	"testing"

	"github.com/paytrack/backend/internal/merchant"
)

func TestHeuristicExtract(t *testing.T) {
	merchant.InitRegistry(".")

	tests := []struct {
		name         string
		subject      string
		sender       string
		body         string
		wantPayment  bool
		wantAmount   float64
		wantCurrency string
		wantMerchant string
	}{
		{
			name:         "HDFC Bank Swiggy UPI Alert",
			subject:      "Alert: Your A/C XX1234 has been debited for Rs 1,450.00",
			sender:       "alerts@hdfcbank.net",
			body:         "Dear Customer, INR 1,450.00 has been debited from your account XX1234 on 02-Oct-2026 towards Swiggy UPI. Available balance: Rs. 15,200.",
			wantPayment:  true,
			wantAmount:   1450.00,
			wantCurrency: "INR",
			wantMerchant: "Swiggy",
		},
		{
			name:         "Myntra Order Confirmation in Rupees",
			subject:      "Order Confirmed: Your Myntra order #128492 of Rs. 2,499 has been placed",
			sender:       "updates@myntra.com",
			body:         "Thank you for shopping at Myntra. Total paid: ₹ 2,499.00 via UPI.",
			wantPayment:  true,
			wantAmount:   2499.00,
			wantCurrency: "INR",
			wantMerchant: "Myntra",
		},
		{
			name:         "Credit Card Charge USD",
			subject:      "Transaction Alert: Your card ending 8842 was charged $49.99",
			sender:       "no-reply@chase.com",
			body:         "A charge of $49.99 at Netflix has been authorized on your card ending in 8842.",
			wantPayment:  true,
			wantAmount:   49.99,
			wantCurrency: "USD",
			wantMerchant: "Netflix",
		},
		{
			name:         "Apple Receipt",
			subject:      "Your receipt from Apple",
			sender:       "no_reply@email.apple.com",
			body:         "Total: $2.99 billed to your Mastercard. Apple iCloud 50GB storage plan.",
			wantPayment:  true,
			wantAmount:   2.99,
			wantCurrency: "USD",
			wantMerchant: "Apple",
		},
		{
			name:         "Non-payment newsletter email",
			subject:      "Weekly Tech Digest - Edition #45",
			sender:       "news@techdaily.com",
			body:         "Here are the top 10 AI frameworks you should know about this week.",
			wantPayment:  false,
			wantAmount:   0,
			wantCurrency: "INR",
			wantMerchant: "Unknown Merchant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := HeuristicExtract(tt.subject, tt.sender, tt.body, "INR")
			if res.IsPayment != tt.wantPayment {
				t.Errorf("IsPayment = %v, want %v", res.IsPayment, tt.wantPayment)
			}
			if tt.wantPayment && res.Amount != tt.wantAmount {
				t.Errorf("Amount = %.2f, want %.2f", res.Amount, tt.wantAmount)
			}
			if tt.wantPayment && res.Currency != tt.wantCurrency {
				t.Errorf("Currency = %s, want %s", res.Currency, tt.wantCurrency)
			}
			if tt.wantPayment && res.Merchant != tt.wantMerchant {
				t.Errorf("Merchant = '%s', want '%s'", res.Merchant, tt.wantMerchant)
			}
		})
	}
}
