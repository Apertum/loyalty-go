package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSubmitOrders_DuplicateByUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// Order already exists for this user → duplicate
	mock.ExpectQuery("SELECT id FROM orders").
		WithArgs("user-123", "123456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("order-abc"))

	client := NewPointsCalcClient("http://test:8080")
	svc := NewOrderService(db, client)

	accepted, duplicates, conflicts, err := svc.SubmitOrders(context.Background(), "user-123", []string{"123456"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(accepted) != 0 {
		t.Errorf("expected 0 accepted, got %d", len(accepted))
	}
	if len(duplicates) != 1 || duplicates[0] != "123456" {
		t.Errorf("expected 1 duplicate [123456], got %v", duplicates)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %v", conflicts)
	}
}

func TestSubmitOrders_EmptyList(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	client := NewPointsCalcClient("http://test:8080")
	svc := NewOrderService(db, client)

	accepted, duplicates, conflicts, err := svc.SubmitOrders(context.Background(), "user-123", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(accepted) != 0 || len(duplicates) != 0 || len(conflicts) != 0 {
		t.Errorf("expected all empty, got accepted=%d dups=%d conflicts=%d", len(accepted), len(duplicates), len(conflicts))
	}
}

func TestGetOrderCalculation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	client := NewPointsCalcClient("http://test:8080")
	svc := NewOrderService(db, client)

	rows := sqlmock.NewRows([]string{"status", "accrual"}).AddRow("PROCESSED", 50)
	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("123456").
		WillReturnRows(rows)

	status, accrual, err := svc.GetOrderCalculation(context.Background(), "123456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != "PROCESSED" {
		t.Errorf("expected status PROCESSED, got %s", status)
	}
	if accrual == nil || *accrual != 50 {
		t.Errorf("expected accrual 50, got %v", accrual)
	}
}

func TestGetOrderCalculation_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	client := NewPointsCalcClient("http://test:8080")
	svc := NewOrderService(db, client)

	mock.ExpectQuery("SELECT status, accrual").
		WithArgs("999999").
		WillReturnError(sql.ErrNoRows)

	_, _, err = svc.GetOrderCalculation(context.Background(), "999999")
	if err == nil {
		t.Error("expected error for not found order")
	}
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestGetDB(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	client := NewPointsCalcClient("http://test:8080")
	svc := NewOrderService(db, client)

	gotDB := svc.GetDB()
	if gotDB == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestRedeemPoints(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	client := NewPointsCalcClient("http://test:8080")
	svc := NewOrderService(db, client)

	// RedeemPoints is a stub, should always return nil
	err = svc.RedeemPoints(context.Background(), "user-123", "123456", 100)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCheckOrder_JsonEscape(t *testing.T) {
	client := NewPointsCalcClient("http://test:8080")

	// Проверка что CheckOrder не падает на строках со спецсимволами
	// json.Marshal корректно экранирует
	points, err := client.CheckOrder(context.Background(), "123\\\"}{}")
	if err == nil {
		_ = points
	}
}
