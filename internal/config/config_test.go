package config

import (
	"os"
	"testing"
)

func TestNewWithValues(t *testing.T) {
	cfg := NewWithValues(":9090", "postgres://test@localhost:5432/test", "http://points:8080")

	if cfg.Address != ":9090" {
		t.Errorf("expected address :9090, got %s", cfg.Address)
	}
	if cfg.DatabaseURI != "postgres://test@localhost:5432/test" {
		t.Errorf("expected database URI, got %s", cfg.DatabaseURI)
	}
	if cfg.AccrualSystemURL != "http://points:8080" {
		t.Errorf("expected accrual system URL, got %s", cfg.AccrualSystemURL)
	}
}

func TestNewWithValues_Empty(t *testing.T) {
	cfg := NewWithValues("", "", "")

	if cfg.Address != "" {
		t.Errorf("expected empty address, got %s", cfg.Address)
	}
}

func TestLoad_DefaultAddress(t *testing.T) {
	os.Unsetenv("RUN_ADDRESS")
	os.Args = []string{"test"}
	cfg := Load()
	if cfg.Address != ":8080" {
		t.Errorf("expected default address :8080, got %s", cfg.Address)
	}
}

func TestLoad_DatabaseURI(t *testing.T) {
	os.Setenv("DATABASE_URI", "postgres://test:test@localhost:5432/testdb")
	defer os.Unsetenv("DATABASE_URI")
	os.Args = []string{"test"}
	cfg := Load()
	if cfg.DatabaseURI != "postgres://test:test@localhost:5432/testdb" {
		t.Errorf("expected DATABASE_URI to be set, got %s", cfg.DatabaseURI)
	}
}

func TestLoad_AccrualSystemURL(t *testing.T) {
	os.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://points-system:8080")
	defer os.Unsetenv("ACCRUAL_SYSTEM_ADDRESS")
	os.Args = []string{"test"}
	cfg := Load()
	if cfg.AccrualSystemURL != "http://points-system:8080" {
		t.Errorf("expected ACCRUAL_SYSTEM_ADDRESS to be set, got %s", cfg.AccrualSystemURL)
	}
}

func TestLoad_AllValues(t *testing.T) {
	os.Setenv("RUN_ADDRESS", ":9090")
	os.Setenv("DATABASE_URI", "postgres://test@localhost:5432/test")
	os.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://points:8080")
	os.Setenv("JWT_SIGNING_KEY", "secret-jwt-key")
	defer func() {
		os.Unsetenv("RUN_ADDRESS")
		os.Unsetenv("DATABASE_URI")
		os.Unsetenv("ACCRUAL_SYSTEM_ADDRESS")
		os.Unsetenv("JWT_SIGNING_KEY")
	}()
	os.Args = []string{"test"}
	cfg := Load()
	if cfg.Address != ":9090" {
		t.Errorf("expected address :9090, got %s", cfg.Address)
	}
	if cfg.DatabaseURI != "postgres://test@localhost:5432/test" {
		t.Errorf("expected database URI, got %s", cfg.DatabaseURI)
	}
	if cfg.AccrualSystemURL != "http://points:8080" {
		t.Errorf("expected accrual system URL, got %s", cfg.AccrualSystemURL)
	}
	if cfg.JWTSigningKey != "secret-jwt-key" {
		t.Errorf("expected JWT signing key, got %s", cfg.JWTSigningKey)
	}
}

func TestLoad_JWTSigningKey(t *testing.T) {
	os.Setenv("JWT_SIGNING_KEY", "env-jwt-secret")
	defer os.Unsetenv("JWT_SIGNING_KEY")
	os.Args = []string{"test"}
	cfg := Load()
	if cfg.JWTSigningKey != "env-jwt-secret" {
		t.Errorf("expected JWT_SIGNING_KEY to be set, got %s", cfg.JWTSigningKey)
	}
}
