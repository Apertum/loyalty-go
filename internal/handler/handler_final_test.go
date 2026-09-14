package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"loyalty-service/internal/jwt"
	"loyalty-service/internal/service"
)

func TestOrderHandler_Submit_DigitsOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewOrderHandler(orderSvc, authSvc)

	// Order exists for this user → returns duplicate
	mock.ExpectQuery("SELECT id FROM orders").
		WithArgs("user-123", "123456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("order-abc"))

	w := httptest.NewRecorder()
	body, _ := json.Marshal([]string{"123456"})
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	h.Submit(w, req)

	// Only duplicates → 200 OK
	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	} else {
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		dups, ok := resp["duplicates"].([]interface{})
		if !ok || len(dups) != 1 {
			t.Errorf("expected 1 duplicate, got %v", resp)
		}
	}
}

func TestBalanceHandler_Balance_Zero(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"float", "int"}).AddRow(0.0, 0)
	mock.ExpectQuery("SELECT COALESCE").
		WithArgs("user-0").
		WillReturnRows(rows)

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewBalanceHandler(authSvc, orderSvc)

	w := httptest.NewRecorder()
	ctx := jwt.ContextWithUser(context.Background(), "user-0", "testuser")
	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	req = req.WithContext(ctx)

	h.Balance(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["current"] != float64(0) {
		t.Errorf("expected current 0, got %v", resp["current"])
	}
}

func TestAuthHandler_Refresh_InvalidToken(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	h := NewAuthHandler(authSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/refresh", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	h.Refresh(w, req)

	if w.Code != 401 {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	h := NewAuthHandler(authSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	h.Login(w, req)

	if w.Code != 400 {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestBalanceHandler_Withdraw_EmptyOrder(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewBalanceHandler(authSvc, orderSvc)

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"order": "",
		"sum":   100,
	})
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	h.Withdraw(w, req)

	if w.Code != 422 {
		t.Errorf("expected status 422, got %d", w.Code)
	}
}

func TestIsDigits_MoreCases(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{"0", true},
		{"0000000000", true},
		{"1", true},
		{"12345678901234567890", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isDigits(tt.input)
			if result != tt.expect {
				t.Errorf("isDigits(%q) = %v, want %v", tt.input, result, tt.expect)
			}
		})
	}
}

func TestOrderHandler_List_NoOrders(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "order_number", "status", "accrual", "uploaded_at", "updated_at",
	})

	mock.ExpectQuery("SELECT id, user_id, order_number").
		WithArgs("user-empty").
		WillReturnRows(rows)

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewOrderHandler(orderSvc, authSvc)

	w := httptest.NewRecorder()
	ctx := jwt.ContextWithUser(context.Background(), "user-empty", "testuser")
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	req = req.WithContext(ctx)

	h.List(w, req)

	if w.Code != 204 {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestBalanceHandler_Withdrawals_Multiple(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "order_id", "order_number", "type", "points", "order_discount", "description", "created_at",
	}).AddRow("tx-1", "user-123", "order-1", "123456", "redeemed", -100, nil, "Redemption", now)

	mock.ExpectQuery("SELECT t.id, t.user_id, t.order_id, o.order_number").
		WithArgs("user-123").
		WillReturnRows(rows)

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewBalanceHandler(authSvc, orderSvc)

	w := httptest.NewRecorder()
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	req = req.WithContext(ctx)

	h.Withdrawals(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestOrderHandler_Calculation_REGISTERED(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"status", "accrual"}).AddRow("REGISTERED", nil)
	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("123456").
		WillReturnRows(rows)

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewOrderHandler(orderSvc, authSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/123456", nil)

	h.Calculation(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "REGISTERED" {
		t.Errorf("expected status REGISTERED, got %v", resp["status"])
	}
	// accrual should not be present for REGISTERED status
	if _, ok := resp["accrual"]; ok {
		t.Error("expected no accrual field for REGISTERED status")
	}
}

func TestOrderHandler_Calculation_NEWMappedToREGISTERED(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// DB returns "NEW" (as stored internally)
	rows := sqlmock.NewRows([]string{"status", "accrual"}).AddRow("NEW", nil)
	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("789012").
		WillReturnRows(rows)

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewOrderHandler(orderSvc, authSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/789012", nil)

	h.Calculation(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "REGISTERED" {
		t.Errorf("expected status REGISTERED (mapped from NEW), got %v", resp["status"])
	}
}

func TestOrderHandler_Calculation_RateLimitExceeded(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)

	// Create handler with low rate limit for testing: 2 requests per minute
	h := &OrderHandler{
		orderService: orderSvc,
		authService:  authSvc,
		rateLimiter:  newCalcRateLimiter(2, time.Minute),
	}

	// First request — should pass (DB expectation 1)
	rows1 := sqlmock.NewRows([]string{"status", "accrual"}).AddRow("PROCESSED", 50)
	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("111111").
		WillReturnRows(rows1)
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/api/orders/111111", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	h.Calculation(w1, req1)
	if w1.Code != 200 {
		t.Errorf("request 1: expected status 200, got %d", w1.Code)
		return
	}

	// Second request — should pass (DB expectation 2)
	rows2 := sqlmock.NewRows([]string{"status", "accrual"}).AddRow("PROCESSED", 50)
	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("111111").
		WillReturnRows(rows2)
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/orders/111111", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	h.Calculation(w2, req2)
	if w2.Code != 200 {
		t.Errorf("request 2: expected status 200, got %d", w2.Code)
		return
	}

	// Third request — should be rate limited (no DB call needed)
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/orders/111111", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	h.Calculation(w3, req3)
	if w3.Code != 429 {
		t.Errorf("request 3: expected status 429, got %d", w3.Code)
		return
	}

	ct := w3.Header().Get("Content-Type")
	if ct != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %s", ct)
	}

	retryAfter := w3.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header to be set")
	}

	body := w3.Body.String()
	if body == "" {
		t.Error("expected non-empty body for rate limit response")
	}

	// Different IP should not be rate limited
	rows4 := sqlmock.NewRows([]string{"status", "accrual"}).AddRow("PROCESSED", 50)
	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("111111").
		WillReturnRows(rows4)
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodGet, "/api/orders/111111", nil)
	req4.RemoteAddr = "10.0.0.1:54321"
	h.Calculation(w4, req4)
	if w4.Code != 200 {
		t.Errorf("different IP: expected status 200, got %d", w4.Code)
	}
}

func TestCalcRateLimiter_Concurrent(t *testing.T) {
	rl := newCalcRateLimiter(5, time.Minute)

	// Launch 10 concurrent requests from the same "IP"
	var wg sync.WaitGroup
	allowedCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _ := rl.Allow("192.168.1.1")
			if ok {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Exactly 5 should be allowed
	if allowedCount != 5 {
		t.Errorf("expected 5 allowed, got %d", allowedCount)
	}
}

func TestCalcRateLimiter_WindowExpiry(t *testing.T) {
	rl := newCalcRateLimiter(2, 50*time.Millisecond)

	// First 2 requests
	ok1, _ := rl.Allow("10.0.0.1")
	ok2, _ := rl.Allow("10.0.0.1")
	if !ok1 || !ok2 {
		t.Error("first 2 requests should be allowed")
	}

	// Third should be rejected
	ok3, _ := rl.Allow("10.0.0.1")
	if ok3 {
		t.Error("third request should be rejected")
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	ok4, _ := rl.Allow("10.0.0.1")
	if !ok4 {
		t.Error("request after window expiry should be allowed")
	}
}
