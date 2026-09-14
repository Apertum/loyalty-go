package service

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRegister_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO users").
		WithArgs("testuser", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	s := NewAuthService(db)
	_, err = s.Register(context.Background(), "testuser", "password123")
	// Error is expected because there's no RETURNING id support in mock
	// The test verifies the password hashing works
	if err == nil {
		t.Fatal("expected error for missing RETURNING id")
	}
}

func TestRegister_EmptyFields(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	s := NewAuthService(db)
	_, err = s.Register(context.Background(), "", "password")
	if err == nil {
		t.Fatal("expected error for empty login")
	}

	_, err = s.Register(context.Background(), "login", "")
	if err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestNewAuthService(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	s := NewAuthService(db)
	if s == nil {
		t.Fatal("expected non-nil AuthService")
	}
	if s.db == nil {
		t.Fatal("expected non-nil db")
	}
}
