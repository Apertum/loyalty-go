package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"loyalty-service/internal/jwt"
)

func TestRefreshToken(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	s := NewAuthService(db)
	// Generate a valid token first
	token, _ := jwt.GenerateJWT("user-123", "testuser")
	claims, err := s.RefreshToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected user-123, got %s", claims.UserID)
	}
}

func TestRefreshToken_Invalid(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	s := NewAuthService(db)
	_, err = s.RefreshToken(context.Background(), "invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestLogout(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	s := NewAuthService(db)
	err = s.Logout(context.Background(), "any-token")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestGetBalance(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT COALESCE").
		WithArgs("user-123").
		WillReturnRows(sqlmock.NewRows([]string{"float", "int"}).AddRow(150.0, 50))

	s := NewAuthService(db)
	balance, err := s.GetBalance(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if balance.Current != 150.0 {
		t.Errorf("expected current 150.0, got %f", balance.Current)
	}
	if balance.Withdrawn != 50 {
		t.Errorf("expected withdrawn 50, got %d", balance.Withdrawn)
	}
}

func TestGetOrders(t *testing.T) {
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

	s := NewAuthService(db)
	orders, err := s.GetOrders(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].OrderNumber != "123456" {
		t.Errorf("expected order 123456, got %s", orders[0].OrderNumber)
	}
}

func TestGetOrders_Empty(t *testing.T) {
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

	s := NewAuthService(db)
	orders, err := s.GetOrders(context.Background(), "user-456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(orders) != 0 {
		t.Errorf("expected 0 orders, got %d", len(orders))
	}
}

func TestGetWithdrawals(t *testing.T) {
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

	s := NewAuthService(db)
	withdrawals, err := s.GetWithdrawals(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(withdrawals) != 1 {
		t.Fatalf("expected 1 withdrawal, got %d", len(withdrawals))
	}
	if withdrawals[0].Points != -100 {
		t.Errorf("expected points -100, got %d", withdrawals[0].Points)
	}
}

func TestGetWithdrawals_Empty(t *testing.T) {
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

	s := NewAuthService(db)
	withdrawals, err := s.GetWithdrawals(context.Background(), "user-456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(withdrawals) != 0 {
		t.Errorf("expected 0 withdrawals, got %d", len(withdrawals))
	}
}
