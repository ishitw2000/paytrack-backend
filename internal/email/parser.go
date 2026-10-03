package email

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// PaymentKeywords are common terms present in transaction confirmation emails
var PaymentKeywords = []string{
	"debited", "credited", "payment", "paid", "spent", "received",
	"transaction", "receipt", "invoice", "order", "statement", "bill",
	"charged", "card ending", "a/c", "acct", "upi", "vpa", "gpay",
	"phonepe", "paytm", "amazon pay", "apple pay", "paypal", "stripe",
	"razorpay", "subscription", "autopay", "transfer", "inr", "usd",
	"eur", "gbp", "rs.", "rs ", "₹", "$", "€", "£",
}

// IsLikelyPaymentEmail checks whether the subject or sender indicates a financial or payment email
func IsLikelyPaymentEmail(subject, sender, body string) bool {
	combined := strings.ToLower(subject + " " + sender)
	for _, kw := range PaymentKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	// If subject doesn't match, check first 500 chars of body
	lowerBody := strings.ToLower(body)
	if len(lowerBody) > 500 {
		lowerBody = lowerBody[:500]
	}
	for _, kw := range PaymentKeywords {
		if strings.Contains(lowerBody, kw) {
			return true
		}
	}
	return false
}

// ExtractTextFromHTML strips HTML tags and produces clean plain text for LLM ingestion
func ExtractTextFromHTML(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		// Fallback to regex if parsing fails
		re := regexp.MustCompile(`<[^>]*>`)
		return cleanWhitespace(re.ReplaceAllString(htmlContent, " "))
	}

	var buf bytes.Buffer
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		// Skip script and style blocks
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "noscript") {
			return
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				buf.WriteString(text)
				buf.WriteString(" ")
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return cleanWhitespace(buf.String())
}

// ReadAllString reads all data from a reader into a string
func ReadAllString(r io.Reader) string {
	if r == nil {
		return ""
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func cleanWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	cleaned := re.ReplaceAllString(s, " ")
	return strings.TrimSpace(cleaned)
}

// TruncateBody limits the email body sent to the LLM to save tokens
func TruncateBody(body string, maxChars int) string {
	if len(body) <= maxChars {
		return body
	}
	return body[:maxChars] + " ...[truncated]"
}
