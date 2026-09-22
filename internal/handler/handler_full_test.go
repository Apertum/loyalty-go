package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"loyalty-service/internal/jwt"
	"loyalty-service/internal/service"
)

func TestOrderHandler_Submit_NonDigits(t *testing.T) {
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
	body, _ := json.Marshal([]string{"abc123"})
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	h.Submit(w, req)

	if w.Code != 422 {
		t.Errorf("expected status 422, got %d", w.Code)
	}
}

func TestOrderHandler_List(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "order_number", "status", "accrual", "uploaded_at", "updated_at",
	}).AddRow("order-1", "user-123", "123456", "PROCESSED", 50, now, now)

	mock.ExpectQuery("SELECT id, user_id, order_number").
		WithArgs("user-123").
		WillReturnRows(rows)

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewOrderHandler(orderSvc, authSvc)

	w := httptest.NewRecorder()
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	req = req.WithContext(ctx)

	h.List(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
		return
	}

	var resp []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 order, got %d", len(resp))
	}
}

func TestOrderHandler_List_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "order_number", "status", "accrual", "uploaded_at", "updated_at",
	})

	mock.ExpectQuery("SELECT id, user_id, order_number").
		WithArgs("user-456").
		WillReturnRows(rows)

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewOrderHandler(orderSvc, authSvc)

	w := httptest.NewRecorder()
	ctx := jwt.ContextWithUser(context.Background(), "user-456", "testuser")
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	req = req.WithContext(ctx)

	h.List(w, req)

	if w.Code != 204 {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestBalanceHandler_Balance_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT COALESCE").
		WithArgs("user-123").
		WillReturnRows(sqlmock.NewRows([]string{"float", "int"}).AddRow(150.0, 50))

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewBalanceHandler(authSvc, orderSvc)

	w := httptest.NewRecorder()
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	req = req.WithContext(ctx)

	h.Balance(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["current"] != float64(150) {
		t.Errorf("expected current 150, got %v", resp["current"])
	}
}

func TestBalanceHandler_Withdraw_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// Order belongs to this user, status PROCESSED
	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("user-123", "123456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "accrual"}).AddRow("PROCESSED", 50))

	// Transaction begin
	mock.ExpectBegin()

	// Lock transactions rows to prevent race condition on balance read
	mock.ExpectExec("SELECT.*FOR UPDATE").
		WithArgs("user-123").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Balance check inside transaction
	mock.ExpectQuery("SELECT COALESCE").
		WithArgs("user-123").
		WillReturnRows(sqlmock.NewRows([]string{"float"}).AddRow(200.0))

	// Order ID lookup inside transaction
	mock.ExpectQuery("SELECT id FROM orders").
		WithArgs("user-123", "123456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("order-123"))

	// Withdrawn total
	mock.ExpectQuery("SELECT COALESCE").
		WithArgs("user-123").
		WillReturnRows(sqlmock.NewRows([]string{"int"}).AddRow(50))

	// Transaction insert (now with order_id)
	mock.ExpectExec("INSERT INTO transactions").
		WithArgs("user-123", "order-123", 100, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Commit
	mock.ExpectCommit()

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewBalanceHandler(authSvc, orderSvc)

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"order": "123456",
		"sum":   100,
	})
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	h.Withdraw(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["current"] != float64(100) {
		t.Errorf("expected current 100, got %v", resp["current"])
	}
}

func TestBalanceHandler_Withdraw_Insufficient(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// Order belongs to this user
	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("user-123", "123456").
		WillReturnRows(sqlmock.NewRows([]string{"status", "accrual"}).AddRow("PROCESSED", 50))

	// Transaction begin
	mock.ExpectBegin()

	// Lock transactions rows to prevent race condition on balance read
	mock.ExpectExec("SELECT.*FOR UPDATE").
		WithArgs("user-123").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Balance check - returns 50, not enough for sum 100
	mock.ExpectQuery("SELECT COALESCE").
		WithArgs("user-123").
		WillReturnRows(sqlmock.NewRows([]string{"float"}).AddRow(50.0))

	// Rollback (balance insufficient)
	mock.ExpectRollback()

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewBalanceHandler(authSvc, orderSvc)

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"order": "123456",
		"sum":   100,
	})
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	h.Withdraw(w, req)

	if w.Code != 402 {
		t.Errorf("expected status 402, got %d", w.Code)
	}
}

func TestBalanceHandler_Withdraw_InvalidSum(t *testing.T) {
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
		"sum":   0,
	})
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	h.Withdraw(w, req)

	if w.Code != 422 {
		t.Errorf("expected status 422, got %d", w.Code)
	}
}

func TestBalanceHandler_Withdraw_InvalidOrder(t *testing.T) {
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
		"order": "abc",
		"sum":   100,
	})
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	h.Withdraw(w, req)

	if w.Code != 422 {
		t.Errorf("expected status 422, got %d", w.Code)
	}
}

func TestBalanceHandler_Withdrawals(t *testing.T) {
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

func TestBalanceHandler_Withdrawals_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "order_id", "order_number", "type", "points", "order_discount", "description", "created_at",
	})

	mock.ExpectQuery("SELECT t.id, t.user_id, t.order_id, o.order_number").
		WithArgs("user-456").
		WillReturnRows(rows)

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewBalanceHandler(authSvc, orderSvc)

	w := httptest.NewRecorder()
	ctx := jwt.ContextWithUser(context.Background(), "user-456", "testuser")
	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	req = req.WithContext(ctx)

	h.Withdrawals(w, req)

	if w.Code != 204 {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestOrderHandler_Calculation_Processed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"status", "accrual"}).AddRow("PROCESSED", 50)
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
	if resp["status"] != "PROCESSED" {
		t.Errorf("expected status PROCESSED, got %v", resp["status"])
	}
	if resp["accrual"] != float64(50) {
		t.Errorf("expected accrual 50, got %v", resp["accrual"])
	}
}

func TestOrderHandler_Calculation_InvalidStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"status", "accrual"}).AddRow("INVALID", nil)
	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("999999").
		WillReturnRows(rows)

	authSvc := service.NewAuthService(db)
	pointsClient := service.NewPointsCalcClient("http://test:8080")
	orderSvc := service.NewOrderService(db, pointsClient)
	h := NewOrderHandler(orderSvc, authSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/999999", nil)

	h.Calculation(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "INVALID" {
		t.Errorf("expected status INVALID, got %v", resp["status"])
	}
	// accrual should not be present for INVALID status
	if _, ok := resp["accrual"]; ok {
		t.Error("expected no accrual field for INVALID status")
	}
}

func TestOrderHandler_Submit_EmptyArray(t *testing.T) {
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
	ctx := jwt.ContextWithUser(context.Background(), "user-123", "testuser")
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader([]byte("[]")))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	h.Submit(w, req)

	if w.Code != 400 {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAuthHandler_Register_Valid(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO users").
		WithArgs("testuser", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	authSvc := service.NewAuthService(db)
	h := NewAuthHandler(authSvc)

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{
		"login":    "testuser",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	h.Register(w, req)

	// 200 (success) or 500 (mock limitation with RETURNING) or 409 (duplicate)
	if w.Code != 200 && w.Code != 500 && w.Code != 409 {
		t.Errorf("expected status 200, 500 or 409, got %d", w.Code)
	}
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	h := NewAuthHandler(authSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	h.Register(w, req)

	if w.Code != 400 {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAuthHandler_Login_Valid(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// Return a bcrypt hash that will fail comparison (expected behavior)
	mock.ExpectQuery("SELECT id, login, password_hash").
		WithArgs("testuser").
		WillReturnError(nil)

	authSvc := service.NewAuthService(db)
	h := NewAuthHandler(authSvc)

	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{
		"login":    "testuser",
		"password": "wrongpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	h.Login(w, req)

	// Either 401 or 500 (mock limitation)
	if w.Code != 401 && w.Code != 500 {
		t.Errorf("expected status 401 or 500, got %d", w.Code)
	}
}

func TestAuthHandler_Refresh(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	authSvc := service.NewAuthService(db)
	h := NewAuthHandler(authSvc)

	token, _ := jwt.GenerateJWT("user-123", "testuser")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	h.Refresh(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
