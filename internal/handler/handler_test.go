package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"loyalty-service/internal/jwt"
	"loyalty-service/internal/middleware"
	"loyalty-service/internal/service"
)

func TestAuthHandler_Register(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	h := NewAuthHandler(authSvc)

	tests := []struct {
		name       string
		body       string
		statusCode int
		errorCode  string
	}{
		{
			name:       "empty body",
			body:       "",
			statusCode: 400,
			errorCode:  "VALIDATION_ERROR",
		},
		{
			name:       "empty login",
			body:       `{"login":"","password":"test"}`,
			statusCode: 400,
			errorCode:  "VALIDATION_ERROR",
		},
		{
			name:       "empty password",
			body:       `{"login":"test","password":""}`,
			statusCode: 400,
			errorCode:  "VALIDATION_ERROR",
		},
		{
			name:       "invalid json",
			body:       `not json`,
			statusCode: 400,
			errorCode:  "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			h.Register(w, req)

			if w.Code != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, w.Code)
			}
		})
	}
}

func TestAuthHandler_Login(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	h := NewAuthHandler(authSvc)

	// Mock user not found → returns sql.ErrNoRows → AuthService returns ErrInvalidCredentials
	mock.ExpectQuery("SELECT id, login, password_hash").
		WithArgs("testuser").
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"login": "testuser", "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	h.Login(w, req)

	// ErrNoRows → ErrInvalidCredentials → 401
	if w.Code != 401 {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	h := NewAuthHandler(authSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/logout", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "any-token"})

	h.Logout(w, req)

	if w.Code != 204 {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestOrderHandler_Submit(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewOrderHandler(orderSvc, authSvc)

	w := httptest.NewRecorder()
	body, _ := json.Marshal([]string{"1234567890"})
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Without auth middleware, this should return 401
	h.Submit(w, req)

	if w.Code != 401 {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestBalanceHandler_Balance(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)

	// Without auth middleware, this should return 401
	h.Balance(w, req)

	if w.Code != 401 {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestBalanceHandler_Withdraw(t *testing.T) {
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
		"order": "123456",
		"sum":   100,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Without auth middleware, this should return 401
	h.Withdraw(w, req)

	if w.Code != 401 {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestOrderHandler_Calculation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewOrderHandler(orderSvc, authSvc)

	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("123456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "accrual"}).AddRow("PROCESSED", 50))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/123456", nil)

	h.Calculation(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["order"] != "123456" {
		t.Errorf("expected order 123456, got %v", resp["order"])
	}
	if resp["status"] != "PROCESSED" {
		t.Errorf("expected status PROCESSED, got %v", resp["status"])
	}
}

func TestOrderHandler_Calculation_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewOrderHandler(orderSvc, authSvc)

	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("999999").
		WillReturnError(sql.ErrNoRows)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/999999", nil)

	h.Calculation(w, req)

	// Order not found in DB → 204 No Content
	if w.Code != 204 {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	middleware.WriteError(w, 400, middleware.ValidationError, "bad request")

	if w.Code != 400 {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestIsDigits(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{"1234567890", true},
		{"", false},
		{"abc", false},
		{"123abc", false},
		{"123-456", false},
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

func TestIsDigits_MaxLength(t *testing.T) {
	// Слишком длинный номер должен отклоняться
	longOrder := ""
	for i := 0; i < 100; i++ {
		longOrder += "1"
	}
	if isDigits(longOrder) {
		t.Error("expected false for order number exceeding max length")
	}

	// Ровно 64 символа — допустимо
	okOrder := ""
	for i := 0; i < 64; i++ {
		okOrder += "1"
	}
	if !isDigits(okOrder) {
		t.Error("expected true for order number at max length")
	}

	// 65 символов — не допустимо
	tooLong := ""
	for i := 0; i < 65; i++ {
		tooLong += "1"
	}
	if isDigits(tooLong) {
		t.Error("expected false for order number over max length")
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	jwt.AuthMiddleware(next).ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	jwt.AuthMiddleware(next).ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should have user context
		ctx := r.Context()
		id, ok := jwt.UserIDFromContext(ctx)
		if !ok {
			t.Error("expected user ID in context")
		}
		_ = id
		w.WriteHeader(http.StatusOK)
	})

	token, _ := jwt.GenerateJWT("user-123", "testuser")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	jwt.AuthMiddleware(next).ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
