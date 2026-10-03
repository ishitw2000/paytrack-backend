package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/paytrack/backend/internal/auth"
	"github.com/paytrack/backend/internal/config"
	"github.com/paytrack/backend/internal/database"
	"github.com/paytrack/backend/internal/models"
	"github.com/paytrack/backend/internal/service"
)

func setupTestServer(t *testing.T) (http.Handler, *models.User, string) {
	db, err := database.InitDB("sqlite", ":memory:", "")
	if err != nil {
		t.Fatalf("failed to init in-memory test db: %v", err)
	}

	cfg := &config.Config{
		Port:          "8080",
		JWTSecret:     "test-secret-key-123",
		EncryptionKey: "test-encryption-key-32-chars-!!",
		CronEnabled:   false,
	}

	syncService := service.NewSyncService(cfg, db)
	analyticsService := service.NewAnalyticsService(db, syncService)
	router := SetupRouter(cfg, db, syncService, analyticsService)

	// Create test user
	user := &models.User{
		Email:    "test@example.com",
		FullName: "Test User",
	}
	db.Create(user)

	token, _ := auth.GenerateToken(user.ID, user.Email, cfg.JWTSecret)

	// Seed test expense for user
	exp := &models.Expense{
		UserID:          user.ID,
		Merchant:        "Starbucks",
		Amount:          6.50,
		Currency:        "USD",
		Category:        "Food & Dining",
		Date:            time.Now(),
		PaymentMethod:   "Credit Card",
		TransactionType: "DEBIT",
		Summary:         "Coffee and pastry",
	}
	db.Create(exp)

	return router, user, token
}

func TestHealthCheck(t *testing.T) {
	router, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestSignupAndLogin(t *testing.T) {
	router, _, _ := setupTestServer(t)

	// Signup
	signupData := map[string]string{
		"email":     "newuser@example.com",
		"password":  "securepassword123",
		"full_name": "New User",
	}
	body, _ := json.Marshal(signupData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 for signup, got %d. Body: %s", w.Code, w.Body.String())
	}

	var authResp AuthResponse
	if err := json.NewDecoder(w.Body).Decode(&authResp); err != nil {
		t.Fatalf("failed to decode auth response: %v", err)
	}
	if authResp.Token == "" {
		t.Fatalf("expected non-empty JWT token")
	}

	// Login
	loginData := map[string]string{
		"email":    "newuser@example.com",
		"password": "securepassword123",
	}
	loginBody, _ := json.Marshal(loginData)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("expected status 200 for login, got %d", loginW.Code)
	}
}

func TestListExpensesAuthenticated(t *testing.T) {
	router, _, token := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/expenses", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp PaginatedExpensesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalCount != 1 {
		t.Errorf("expected 1 expense, got %d", resp.TotalCount)
	}
}

func TestTenantIsolation(t *testing.T) {
	router, _, tokenUserA := setupTestServer(t)

	// Create user B
	signupData := map[string]string{
		"email":     "userB@example.com",
		"password":  "password123",
		"full_name": "User B",
	}
	body, _ := json.Marshal(signupData)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var authResp AuthResponse
	_ = json.NewDecoder(w.Body).Decode(&authResp)
	tokenUserB := authResp.Token

	// User B requests expenses -> should be 0 (User A's Starbucks expense is hidden)
	reqB := httptest.NewRequest(http.MethodGet, "/api/v1/expenses", nil)
	reqB.Header.Set("Authorization", "Bearer "+tokenUserB)
	wB := httptest.NewRecorder()
	router.ServeHTTP(wB, reqB)

	var respB PaginatedExpensesResponse
	_ = json.NewDecoder(wB.Body).Decode(&respB)

	if respB.TotalCount != 0 {
		t.Errorf("data leak: User B saw %d expenses, expected 0", respB.TotalCount)
	}

	// User A requests expenses -> should still see their 1 expense
	reqA := httptest.NewRequest(http.MethodGet, "/api/v1/expenses", nil)
	reqA.Header.Set("Authorization", "Bearer "+tokenUserA)
	wA := httptest.NewRecorder()
	router.ServeHTTP(wA, reqA)

	var respA PaginatedExpensesResponse
	_ = json.NewDecoder(wA.Body).Decode(&respA)
	if respA.TotalCount != 1 {
		t.Errorf("User A expected 1 expense, got %d", respA.TotalCount)
	}
}
