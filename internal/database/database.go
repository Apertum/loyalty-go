// Package database предоставляет подключение к PostgreSQL и управление миграциями.
package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sirupsen/logrus"
)

// Open устанавливает соединение с PostgreSQL с использованием заданного DSN.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

// RunMigrations применяет все неприменённые миграции БД из указанной директории.
func RunMigrations(dsn, migrationsPath string) error {
	m, err := migrate.New(
		"file://"+migrationsPath,
		dsn,
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil {
		if err != migrate.ErrNoChange {
			return fmt.Errorf("run migrations: %w", err)
		}
		logrus.Info("нет новых миграций для применения")
	}

	return nil
}

// SetConnMaxLifetime настраирует максимальное время повторного использования подключения.
func SetConnMaxLifetime(db *sql.DB, lifetime time.Duration) {
	db.SetConnMaxLifetime(lifetime)
}

// SetMaxOpenConns настраирует максимальное количество открытых подключений к БД.
func SetMaxOpenConns(db *sql.DB, max int) {
	db.SetMaxOpenConns(max)
}

// SetMaxIdleConns настраирует максимальное количество простаивающих подключений в пуле.
func SetMaxIdleConns(db *sql.DB, max int) {
	db.SetMaxIdleConns(max)
}
