package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"loyalty-service/internal/models"
)

func TestSubmitOrders_Conflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// Order already exists for this user
	mock.ExpectQuery("SELECT id FROM orders").
		WithArgs("user-123", "123456").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("existing-order"))

	client := NewPointsCalcClient("http://test:8080")
	svc := NewOrderService(db, client)

	accepted, dups, conflicts, err := svc.SubmitOrders(context.Background(), "user-123", []string{"123456"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(accepted) != 0 {
		t.Errorf("expected 0 accepted, got %d", len(accepted))
	}
	if len(dups) != 1 {
		t.Errorf("expected 1 duplicate, got %d", len(dups))
	}
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(conflicts))
	}
}

func TestSubmitOrders_Empty(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	client := NewPointsCalcClient("http://test:8080")
	svc := NewOrderService(db, client)

	accepted, dups, conflicts, err := svc.SubmitOrders(context.Background(), "user-123", []string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(accepted) != 0 || len(dups) != 0 || len(conflicts) != 0 {
		t.Errorf("expected all empty, got accepted=%d dups=%d conflicts=%d", len(accepted), len(dups), len(conflicts))
	}
}

func TestGetOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT id, user_id, order_number, status, accrual, uploaded_at, updated_at").
		WithArgs("user-123", "123456").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "order_number", "status", "accrual", "uploaded_at", "updated_at",
		}).AddRow("order-1", "user-123", "123456", "NEW", nil, now, now))

	s := NewAuthService(db)
	order, err := s.GetOrder(context.Background(), "user-123", "123456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if order.OrderNumber != "123456" {
		t.Errorf("expected order 123456, got %s", order.OrderNumber)
	}
	if order.Status != models.OrderStatusNew {
		t.Errorf("expected status NEW, got %s", order.Status)
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, user_id, order_number").
		WithArgs("user-123", "999999").
		WillReturnError(nil)

	s := NewAuthService(db)
	_, err = s.GetOrder(context.Background(), "user-123", "999999")
	if err == nil {
		t.Fatal("expected error for nonexistent order")
	}
}
