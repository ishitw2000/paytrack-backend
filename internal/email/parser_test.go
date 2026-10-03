package email

import (
	"testing"
)

func TestExtractTextFromHTML(t *testing.T) {
	htmlInput := `
		<html>
			<head><title>Receipt</title></head>
			<body>
				<style>.hide { display: none; }</style>
				<h1>Payment Confirmation</h1>
				<p>Your payment of <b>$24.50</b> at <i>Uber Eats</i> was successful.</p>
				<script>console.log("tracking");</script>
			</body>
		</html>
	`

	got := ExtractTextFromHTML(htmlInput)
	expectedSubstrings := []string{"Payment Confirmation", "Your payment of", "$24.50", "Uber Eats", "was successful."}

	for _, sub := range expectedSubstrings {
		if !contains(got, sub) {
			t.Errorf("expected extracted text to contain %q, got %q", sub, got)
		}
	}

	if contains(got, "console.log") {
		t.Errorf("expected script content to be stripped out")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || index(s, substr) >= 0)
}

func index(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
