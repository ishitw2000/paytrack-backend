package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndValidateToken_Success(t *testing.T) {
	secret := "super-secure-production-secret-key-32b!"
	userID := uint(101)
	email := "testuser@example.com"

	tokenStr, err := GenerateToken(userID, email, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if tokenStr == "" {
		t.Fatalf("expected non-empty token string")
	}

	claims, err := ValidateToken(tokenStr, secret)
	if err != nil {
		t.Fatalf("ValidateToken failed on valid token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("UserID = %d, want %d", claims.UserID, userID)
	}

	if claims.Email != email {
		t.Errorf("Email = %s, want %s", claims.Email, email)
	}

	if claims.Issuer != "paytrack-backend" {
		t.Errorf("Issuer = %s, want paytrack-backend", claims.Issuer)
	}

	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(time.Now().Add(29*24*time.Hour)) {
		t.Errorf("ExpiresAt should be ~30 days in the future, got %v", claims.ExpiresAt)
	}
}

func TestValidateToken_InvalidSecret(t *testing.T) {
	correctSecret := "correct-secret-key-abc-123456789"
	wrongSecret := "wrong-secret-key-xyz-987654321"

	tokenStr, err := GenerateToken(1, "user@test.com", correctSecret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(tokenStr, wrongSecret)
	if err == nil {
		t.Errorf("expected validation to fail with wrong secret, but succeeded")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	secret := "secret-key-for-expired-test-token"

	// Construct an expired token directly
	expiredClaims := JWTClaims{
		UserID: 42,
		Email:  "expired@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // 1 hour ago
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "paytrack-backend",
		},
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	tokenStr, err := expiredToken.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to create expired token: %v", err)
	}

	_, err = ValidateToken(tokenStr, secret)
	if err == nil {
		t.Errorf("expected error for expired token, got nil")
	}
}

func TestValidateToken_TamperedSignature(t *testing.T) {
	secret := "secret-for-tamper-testing-123456"
	tokenStr, err := GenerateToken(5, "tamper@example.com", secret)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Split JWT into header.payload.signature
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts in JWT, got %d", len(parts))
	}

	// Tamper with signature
	tamperedSig := parts[2] + "xyz"
	tamperedToken := parts[0] + "." + parts[1] + "." + tamperedSig

	_, err = ValidateToken(tamperedToken, secret)
	if err == nil {
		t.Errorf("expected validation failure for tampered signature, got nil")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	secret := "test-secret"
	malformedTokens := []string{
		"",
		"not-a-token",
		"header.payload", // missing signature
		"header.payload.signature.extra",
		"....",
	}

	for _, token := range malformedTokens {
		_, err := ValidateToken(token, secret)
		if err == nil {
			t.Errorf("expected error for malformed token '%s', got nil", token)
		}
	}
}

func TestValidateToken_RejectsNoneAlgorithm(t *testing.T) {
	secret := "secret-key-32-chars-long-minimum"

	// Create token with "none" signing method
	claims := JWTClaims{
		UserID: 99,
		Email:  "hacker@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to sign token with none: %v", err)
	}

	// ValidateToken should explicitly reject non-HMAC methods
	_, err = ValidateToken(tokenStr, secret)
	if err == nil {
		t.Errorf("expected rejection of 'none' algorithm token, but got success!")
	}
}
