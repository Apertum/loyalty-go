package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLogin_UserNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, login, password_hash").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	s := NewAuthService(db)
	_, err = s.Login(context.Background(), "nonexistent", "password")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestNewPointsCalcClient_Defaults(t *testing.T) {
	client := NewPointsCalcClient("http://test:8080")
	if client.baseURL != "http://test:8080" {
		t.Errorf("expected baseURL http://test:8080, got %s", client.baseURL)
	}
	if client.httpClient == nil {
		t.Fatal("expected non-nil httpClient")
	}
	if client.httpClient.Timeout == 0 {
		t.Error("expected non-zero timeout (should be 5s)")
	}
}
