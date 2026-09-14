// Package config предоставляет конфигурацию приложения через переменные окружения и флаги CLI.
//
// Конфигурация поддерживает следующие параметры:
//   - Адрес сервиса (RUN_ADDRESS / -a): адрес и порт для HTTP-сервера (по умолчанию :8080)
//   - URI базы данных (DATABASE_URI / -d): connection string для PostgreSQL
//   - Адрес системы расчёта баллов (ACCRUAL_SYSTEM_ADDRESS / -r): базовый URL внешней системы баллов
//
// Флаги CLI имеют приоритет над переменными окружения.
package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"unicode"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config содержит все параметры конфигурации приложения.
type Config struct {
	Address          string
	DatabaseURI      string
	AccrualSystemURL string
	JWTSigningKey    string
}

// Load читает конфигурацию из переменных окружения и флагов CLI.
// Флаги CLI имеют приоритет над переменными окружения.
func Load() *Config {
	flagSet := flag.NewFlagSet("loyalty-service", flag.ContinueOnError)
	addrFlag := flagSet.String("a", "", "Адрес сервиса (переопределяет RUN_ADDRESS)")
	dbFlag := flagSet.String("d", "", "Connection string базы данных (переопределяет DATABASE_URI)")
	accrualFlag := flagSet.String("r", "", "Адрес системы расчёта баллов (переопределяет ACCRUAL_SYSTEM_ADDRESS)")
	jwtKeyFlag := flagSet.String("j", "", "Секретный ключ JWT (переопределяет JWT_SIGNING_KEY)")

	// Флаги CLI переопределяют переменные окружения
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		// Игнорируем ошибки парсинга для тестирования
		_ = err
	}

	cfg := &Config{
		Address:          os.Getenv("RUN_ADDRESS"),
		DatabaseURI:      os.Getenv("DATABASE_URI"),
		AccrualSystemURL: os.Getenv("ACCRUAL_SYSTEM_ADDRESS"),
		JWTSigningKey:    os.Getenv("JWT_SIGNING_KEY"),
	}

	// Флаги CLI переопределяют переменные окружения
	if *addrFlag != "" {
		cfg.Address = *addrFlag
	}
	if cfg.Address == "" {
		cfg.Address = ":8080"
	}

	if *dbFlag != "" {
		cfg.DatabaseURI = *dbFlag
	}

	if *accrualFlag != "" {
		cfg.AccrualSystemURL = *accrualFlag
	}
	if *jwtKeyFlag != "" {
		cfg.JWTSigningKey = *jwtKeyFlag
	}

	return cfg
}

// NewWithValues создаёт Config с указанными значениями напрямую.
// Полезно для тестирования без парсинга CLI.
func NewWithValues(address, dbURI, accrualURL string) *Config {
	return &Config{
		Address:          address,
		DatabaseURI:      dbURI,
		AccrualSystemURL: accrualURL,
	}
}

func LogsInit() {

	// установим уровень логирования
	logrus.SetLevel(logrus.TraceLevel)

	// установим форматирование логов для консоли: 2026-07-12T12:00:00Z [info] "msg"
	logrus.SetFormatter(&consoleFormatter{})

	logFile := "logs/app.log"
	// 1. Создаем папку для логов, если её нет
	err := os.MkdirAll("./logs", 0755)
	if err != nil {
		logrus.WithError(err).Fatal("Не удалось создать директорию для логов")
	}

	// 2. Настраиваем lumberjack для ротации логов (лимит 10MB на файл)
	logger := &lumberjack.Logger{
		Filename:  logFile,
		MaxSize:   1,    // максимум 1 МБ на файл
		MaxAge:    7,    // хранить до 7 старых файлов
		Compress:  true, // сжимать старые файлы
		LocalTime: true, // использовать локальное время в именах
	}
	if err := logger.Close(); err != nil {
		logrus.WithError(err).Warn("Ошибка при инициализации lumberjack")
	}

	// 3. Добавляем хук, который дублирует логи в файл с ротацией в JSON формате
	logrus.AddHook(&jsonFileHook{
		Writer: logger,
		LogLevels: []logrus.Level{
			logrus.PanicLevel,
			logrus.FatalLevel,
			logrus.ErrorLevel,
			logrus.WarnLevel,
			logrus.InfoLevel,
			//logrus.DebugLevel,
			//logrus.TraceLevel,
		},
	})
	logrus.Info("Удалось настроить файл логов с ротацией (10MB limit)!")
}

// consoleFormatter — форматер для консоли: 2026-07-12T12:00:00Z [info] "msg"
type consoleFormatter struct{}

func (f *consoleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	level := entry.Level
	levelStr := level.String()
	if levelStr == "" {
		levelStr = "info"
	}
	msg := entry.Message
	timestamp := entry.Time.Format("02.01_15:04:05.000") // психоделика
	return []byte(fmt.Sprintf("%s [%s] \"%s\"\n", timestamp, string(unicode.ToUpper([]rune(levelStr)[0])), msg)), nil
}

// jsonFileHook — хук для logrus, который пишет логи в файл в JSON формате
type jsonFileHook struct {
	Writer    io.Writer
	LogLevels []logrus.Level
}

func (h *jsonFileHook) Levels() []logrus.Level {
	return h.LogLevels
}

func (h *jsonFileHook) Fire(entry *logrus.Entry) error {
	formatter := &logrus.JSONFormatter{}
	bytes, err := formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = h.Writer.Write(bytes)
	return err
}
