package llm

import (
	"strings"
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
		wantTxnType  string
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
			wantTxnType:  "DEBIT",
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
			wantTxnType:  "DEBIT",
		},
		{
			name:         "Credit Card Charge USD Chase",
			subject:      "Transaction Alert: Your card ending 8842 was charged $49.99",
			sender:       "no-reply@chase.com",
			body:         "A charge of $49.99 at Netflix has been authorized on your card ending in 8842.",
			wantPayment:  true,
			wantAmount:   49.99,
			wantCurrency: "USD",
			wantMerchant: "Netflix",
			wantTxnType:  "DEBIT",
		},
		{
			name:         "Apple iCloud Subscription Receipt",
			subject:      "Your receipt from Apple",
			sender:       "no_reply@email.apple.com",
			body:         "Total: $2.99 billed to your Mastercard. Apple iCloud 50GB storage plan subscription.",
			wantPayment:  true,
			wantAmount:   2.99,
			wantCurrency: "USD",
			wantMerchant: "Apple",
			wantTxnType:  "SUBSCRIPTION",
		},
		{
			name:         "ICICI Bank Credit Card Alert Amazon",
			subject:      "Transaction alert for your ICICI Bank Credit Card",
			sender:       "credit_cards@icicibank.com",
			body:         "INR 3,249.00 spent on ICICI Bank Card ending 4012 at Amazon on 14-Oct-26. Available limit: INR 95,000.",
			wantPayment:  true,
			wantAmount:   3249.00,
			wantCurrency: "INR",
			wantMerchant: "Amazon",
			wantTxnType:  "DEBIT",
		},
		{
			name:         "Zomato Food Delivery Alert",
			subject:      "Order Placed with Zomato",
			sender:       "noreply@zomato.com",
			body:         "Your order from Zomato for ₹ 385.50 is confirmed. Paid via Google Pay UPI.",
			wantPayment:  true,
			wantAmount:   385.50,
			wantCurrency: "INR",
			wantMerchant: "Zomato",
			wantTxnType:  "DEBIT",
		},
		{
			name:         "Uber Ride Receipt",
			subject:      "Your Tuesday evening trip with Uber",
			sender:       "uber.india@uber.com",
			body:         "Total: ₹240.00. Thanks for riding with Uber. Paid via Paytm.",
			wantPayment:  true,
			wantAmount:   240.00,
			wantCurrency: "INR",
			wantMerchant: "Uber",
			wantTxnType:  "DEBIT",
		},
		{
			name:         "Refund Credited Transaction",
			subject:      "Credit Alert: INR 450.00 credited to account XX5678",
			sender:       "alerts@sbi.co.in",
			body:         "Dear Customer, INR 450.00 has been credited to your account XX5678 on 15-Oct-2026 towards refund from Swiggy.",
			wantPayment:  true,
			wantAmount:   450.00,
			wantCurrency: "INR",
			wantMerchant: "Swiggy",
			wantTxnType:  "CREDIT",
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
		{
			name:         "Security OTP email (not a payment)",
			subject:      "Your One-Time Password (OTP) for Login",
			sender:       "auth@securebank.com",
			body:         "Your verification code is 482910. Please do not share your OTP with anyone.",
			wantPayment:  false,
			wantAmount:   0,
			wantCurrency: "INR",
		},
		{
			name:         "Account statement notification (no transaction)",
			subject:      "Your Monthly Account Statement is Ready",
			sender:       "statements@bank.com",
			body:         "Your e-statement for the month is attached. Thank you for banking with us.",
			wantPayment:  false,
			wantAmount:   0,
			wantCurrency: "INR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := HeuristicExtract(tt.subject, tt.sender, tt.body, "INR")
			if res.IsPayment != tt.wantPayment {
				t.Errorf("IsPayment = %v, want %v", res.IsPayment, tt.wantPayment)
			}
			if tt.wantPayment {
				if res.Amount != tt.wantAmount {
					t.Errorf("Amount = %.2f, want %.2f", res.Amount, tt.wantAmount)
				}
				if res.Currency != tt.wantCurrency {
					t.Errorf("Currency = %s, want %s", res.Currency, tt.wantCurrency)
				}
				if tt.wantMerchant != "" && !strings.EqualFold(res.Merchant, tt.wantMerchant) {
					t.Errorf("Merchant = '%s', want '%s'", res.Merchant, tt.wantMerchant)
				}
				if tt.wantTxnType != "" && res.TransactionType != tt.wantTxnType {
					t.Errorf("TransactionType = '%s', want '%s'", res.TransactionType, tt.wantTxnType)
				}
			}
		})
	}
}

func BenchmarkHeuristicExtract(b *testing.B) {
	merchant.InitRegistry(".")
	subject := "Alert: Your A/C XX1234 has been debited for Rs 1,450.00"
	sender := "alerts@hdfcbank.net"
	body := "Dear Customer, INR 1,450.00 has been debited from your account XX1234 on 02-Oct-2026 towards Swiggy UPI. Available balance: Rs. 15,200."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := HeuristicExtract(subject, sender, body, "INR")
		if !res.IsPayment {
			b.Fatal("expected payment")
		}
	}
}
