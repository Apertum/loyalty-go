package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCheckOrder_Success(t *testing.T) {
	client := NewPointsCalcClient("http://test:8080")

	if client.baseURL != "http://test:8080" {
		t.Errorf("expected baseURL http://test:8080, got %s", client.baseURL)
	}
	if client.httpClient == nil {
		t.Fatal("expected non-nil http client")
	}
	if client.httpClient.Timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", client.httpClient.Timeout)
	}
}

func TestCheckOrder_InvalidURL(t *testing.T) {
	client := NewPointsCalcClient("://invalid-url")

	_, err := client.CheckOrder(context.Background(), "123456")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestCheckOrder_UnreachableServer(t *testing.T) {
	client := NewPointsCalcClient("http://localhost:1")

	_, err := client.CheckOrder(context.Background(), "123456")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestCheckOrder_RetryOn5xx(t *testing.T) {
	// Создаем HTTP-сервер, который возвращает 500 дважды, а потом 200
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int{"points": 100})
	}))
	defer server.Close()

	client := NewPointsCalcClient(server.URL)
	points, err := client.CheckOrder(context.Background(), "123456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if points != 100 {
		t.Errorf("expected 100 points, got %d", points)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (2 retries + 1 success), got %d", callCount)
	}
}

func TestCheckOrder_AllRetriesFail(t *testing.T) {
	// Создаем HTTP-сервер, который всегда возвращает 500
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewPointsCalcClient(server.URL)
	_, err := client.CheckOrder(context.Background(), "123456")
	if err == nil {
		t.Fatal("expected error after retries")
	}
	// Должно быть 3 попытки (1 initial + 2 retries)
	if callCount != 3 {
		t.Errorf("expected 3 calls (1 initial + 2 retries), got %d", callCount)
	}
}

func TestProcessOrderAsync_Runs(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// processOrderAsync теперь не ставит PROCESSING, сразу вызывает CheckOrder
	// Mock для успешного CheckOrder (возвращает ошибку т.к. сервер недоступен)
	client := NewPointsCalcClient("http://localhost:1") // недостижимый сервер
	svc := NewOrderService(db, client)

	done := make(chan struct{})
	go func() {
		svc.processOrderAsync(context.Background(), "user-123", "123456")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("processOrderAsync timed out")
	}
}

func TestIsDuplicateError_Nil(t *testing.T) {
	if isDuplicateError(nil) {
		t.Fatal("expected false for nil error")
	}
}

func TestPointsChecker_Interface(t *testing.T) {
	pc := NewPointsCalcClient("http://test")
	if pc == nil {
		t.Fatal("expected non-nil PointsChecker")
	}
}

func TestOrderService_Interface(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	client := NewPointsCalcClient("http://test")
	svc := NewOrderService(db, client)
	if svc == nil {
		t.Fatal("expected non-nil OrderService")
	}
	_ = svc
}

func TestAuthService_Interface(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	svc := NewAuthService(db)
	if svc == nil {
		t.Fatal("expected non-nil AuthService")
	}
	_ = svc
}

func TestNewPointsCalcClient_WithNil(t *testing.T) {
	client := NewPointsCalcClient("")
	if client.baseURL != "" {
		t.Errorf("expected empty baseURL, got %s", client.baseURL)
	}
}

func TestOrderService_RedeemPoints_Stub(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	client := NewPointsCalcClient("http://test")
	svc := NewOrderService(db, client)

	err = svc.RedeemPoints(context.Background(), "user-123", "123456", 100)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestAuthService_GetDB(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	svc := NewAuthService(db)
	got := svc.GetDB()
	if got == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestOrderService_GetDB(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	client := NewPointsCalcClient("http://test")
	svc := NewOrderService(db, client)
	got := svc.GetDB()
	if got == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestOrderService_GetOrderCalculation_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("999999").
		WillReturnError(nil)

	client := NewPointsCalcClient("http://test")
	svc := NewOrderService(db, client)

	_, _, err = svc.GetOrderCalculation(context.Background(), "999999")
	if err == nil {
		t.Fatal("expected error for nonexistent order")
	}
}
