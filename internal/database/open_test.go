package database

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func TestOpen_InvalidDSN(t *testing.T) {
	_, err := Open("invalid-dsn")
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestOpen_EmptyDSN(t *testing.T) {
	_, err := Open("")
	if err == nil {
		t.Fatal("expected error for empty DSN")
	}
}

func TestOpen_MalformedDSN(t *testing.T) {
	_, err := Open("not-a-valid-dsn-at-all")
	if err == nil {
		t.Fatal("expected error for malformed DSN")
	}
}

func TestRunMigrations_Unreachable(t *testing.T) {
	err := RunMigrations("postgres://invalid:invalid@localhost:1/test", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unreachable DB")
	}
}

func TestSetConnMaxLifetime(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/test?sslmode=disable")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	// SetConnMaxLifetime sets the connection max lifetime
	SetConnMaxLifetime(db, 10*time.Minute)

	// Verify by getting stats - MaxOpenConnections reflects the setting
	stats := db.Stats()
	_ = stats.MaxOpenConnections // just verify it doesn't panic
}

func TestSetMaxOpenConns(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/test?sslmode=disable")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	SetMaxOpenConns(db, 25)
	stats := db.Stats()
	if stats.MaxOpenConnections != 25 {
		t.Errorf("expected 25, got %d", stats.MaxOpenConnections)
	}
}

func TestSetMaxIdleConns(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/test?sslmode=disable")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	SetMaxIdleConns(db, 5)
	// Stats() doesn't expose MaxIdleConns directly, but we can verify SetMaxIdleConns doesn't panic
	stats := db.Stats()
	_ = stats
}
