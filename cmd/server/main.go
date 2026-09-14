// Package main — точка входа для HTTP-сервиса лояльности.
//
// Loyalty service предоставляет REST API для управления программой лояльности.
// Пользователи могут регистрироваться, входить в систему, отправлять заказы,
// начислять баллы и списывать баллы для получения скидки по заказу.
// Сервис интегрируется с внешней системой расчёта баллов для определения
// вознаграждений за отправленные заказы.
package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"loyalty-service/internal/config"
	"loyalty-service/internal/database"
	"loyalty-service/internal/jwt"
	"loyalty-service/internal/service"
)

func init() {
	config.LogsInit()
	logo()
}

func logo() {
	logrus.Info("██╗      ██████╗ ██╗   ██╗ █████╗ ██╗  ████████╗██╗   ██╗  ")
	logrus.Info("██║     ██╔═══██╗╚██╗ ██╔╝██╔══██╗██║  ╚══██╔══╝╚██╗ ██╔╝  ")
	logrus.Info("██║     ██║   ██║ ╚████╔╝ ███████║██║     ██║    ╚████╔╝   ")
	logrus.Info("██║     ██║   ██║  ╚██╔╝  ██╔══██║██║     ██║     ╚██╔╝    ")
	logrus.Info("███████╗╚██████╔╝   ██║   ██║  ██║███████╗██║      ██║     ")
	logrus.Info("╚══════╝ ╚═════╝    ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝      ╚═╝     ")
}

func main() {
	cfg := config.Load()
	jwt.SetSigningKey(cfg.JWTSigningKey)
	logrus.WithField("address", cfg.Address).Info("загрузка конфигурации")

	if cfg.DatabaseURI == "" {
		logrus.Fatal("DATABASE_URI environment variable or -d flag is required")
	}

	db, err := database.Open(cfg.DatabaseURI)
	if err != nil {
		logrus.WithError(err).Fatal("не удалось открыть базу данных")
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			logrus.WithError(err).Fatal("не удалось закрыть базу данных")
		}
	}(db)

	database.SetConnMaxLifetime(db, 10*time.Minute)
	database.SetMaxOpenConns(db, 25)
	database.SetMaxIdleConns(db, 5)

	migrationsPath := "migrations"
	if err := database.RunMigrations(cfg.DatabaseURI, migrationsPath); err != nil {
		logrus.WithError(err).Fatal("не удалось применить миграции")
	}

	authService := service.NewAuthService(db)

	r := setupRoutes(db, authService, cfg.AccrualSystemURL)

	addr := cfg.Address
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	done := make(chan struct{})
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logrus.Info("остановка сервера...")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logrus.WithError(err).Error("ошибка остановки сервера")
		}
		close(done)
	}()

	logrus.WithField("addr", addr).Info("запуск сервиса лояльности")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logrus.WithError(err).Fatal("сервер не смог запуститься")
	}

	<-done
	logrus.Info("сервер остановлен")
}
