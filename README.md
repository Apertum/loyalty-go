# loyalty-service

HTTP API сервис для управления системой лояльности «Гофермарт» на Go.

## О проекте

Сервис предоставляет REST API для управления программой лояльности:
- Регистрация и аутентификация пользователей (JWT)
- Приём и учёт номеров заказов
- Интеграция с внешней системой расчёта баллов (PointsCalcSystem)
- Накопительный счёт с начислением и списанием баллов
- API для получения информации о расчёте начислений

## Требования

- Go 1.26.4+
- PostgreSQL 14+
- GolangCI-Lint (для проверки кода)

## Установка

```bash
# Клонировать репозиторий
git clone <repository-url>
cd loyalty-service

# Установить зависимости
go mod download

# Проверить сборку
go build ./...
```

## Компиляция

### Linux / macOS

```bash
# Скомпилировать бинарный файл
go build -o bin/loyalty-service ./cmd/server/

# Скомпилировать с флагом оптимизации (управление размером бинарного файла)
go build -ldflags="-s -w" -o bin/loyalty-service ./cmd/server/

# Скомпилировать кроссплатформенно
GOOS=linux GOARCH=amd64 go build -o bin/loyalty-service-linux-amd64 ./cmd/server/
GOOS=windows GOARCH=amd64 go build -o bin/loyalty-service-windows.exe ./cmd/server/
GOOS=darwin GOARCH=amd64 go build -o bin/loyalty-service-darwin ./cmd/server/
```

### Windows

```cmd
:: Скомпилировать бинарный файл
go build -o bin\loyalty-service.exe .\cmd\server\

:: Скомпилировать с флагом оптимизации
go build -ldflags="-s -w" -o bin\loyalty-service.exe .\cmd\server\

:: Кроссплатформенная компиляция
set GOOS=linux
set GOARCH=amd64
go build -o bin\loyalty-service-linux.exe .\cmd\server\
set GOOS=
set GOARCH=

:: Скомпилировать бинарный файл под macOS (Apple Silicon)
set GOOS=darwin
set GOARCH=arm64
go build -o bin\loyalty-service-darwin .\cmd\server\
set GOOS=
set GOARCH=
```

## Запуск

### Linux

```bash
# Запустить через переменные окружения
export RUN_ADDRESS=":8080"
export DATABASE_URI="postgres://user:password@localhost:5432/loyalty_db?sslmode=disable"
export ACCRUAL_SYSTEM_ADDRESS="http://points-system.internal"
./bin/loyalty-service

# Запустить через флаги
./bin/loyalty-service -a :9090 -d "postgres://user:password@localhost:5432/loyalty_db" -r "http://points-system:8080"

# Запустить в фоне (nohup)
nohup ./bin/loyalty-service > loyalty-service.log 2>&1 &

# Запустить через systemd (пример unit-файла)
sudo tee /etc/systemd/system/loyalty-service.service <<EOF
[Unit]
Description=Loyalty Service
After=network.target postgresql.service

[Service]
Type=simple
User=loyalty
WorkingDirectory=/opt/loyalty-service
ExecStart=/opt/loyalty-service/bin/loyalty-service -a :8080 -d "postgres://user:pass@localhost:5432/loyalty_db" -r "http://points-system:8080"
Restart=on-failure
RestartSec=5
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable loyalty-service
sudo systemctl start loyalty-service
```

### Windows

```cmd
:: Запустить через переменные окружения
set RUN_ADDRESS=:8080
set DATABASE_URI=postgres://user:password@localhost:5432/loyalty_db?sslmode=disable
set ACCRUAL_SYSTEM_ADDRESS=http://points-system.internal
bin\loyalty-service.exe

:: Запустить через флаги
bin\loyalty-service.exe -a :9090 -d "postgres://user:password@localhost:5432/loyalty_db" -r "http://points-system:8080"

:: Запустить в фоне (PowerShell)
Start-Process -FilePath "bin\loyalty-service.exe" -ArgumentList "-a", ":8080", "-d", "postgres://user:pass@localhost:5432/db", "-r", "http://points:8080" -WindowStyle Hidden
```

## Тестовые запросы (curl)

### Аутентификация

**Регистрация пользователя:**

```bash
curl -X POST http://localhost:8080/api/user/register \
  -H "Content-Type: application/json" \
  -d '{"login": "ivan.petrov", "password": "SecurePass123!"}' \
  -v
```

**Ответ (200 OK):**

```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "login": "ivan.petrov"
}
```

**Вход в систему:**

```bash
curl -X POST http://localhost:8080/api/user/login \
  -H "Content-Type: application/json" \
  -d '{"login": "ivan.petrov", "password": "SecurePass123!"}' \
  -v
```

**Ответ (200 OK):**

```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "login": "ivan.petrov"
}
```

**Обновление токена:**

```bash
# Получение токена из cookie
curl -X POST http://localhost:8080/api/user/refresh \
  -c cookies.txt \
  -b cookies.txt \
  -v
```

**Выход из системы:**

```bash
curl -X POST http://localhost:8080/api/user/logout \
  -b cookies.txt \
  -v
```

### Заказы

**Загрузка номеров заказов:**

```bash
# Загрузить несколько номеров заказов
curl -X POST http://localhost:8080/api/user/orders \
  -b cookies.txt \
  -H "Content-Type: application/json" \
  -d '[1234567890, 9876543210, 5555666677]' \
  -v
```

**Ответ (202 Accepted):**

```json
{
  "accepted": ["1234567890", "9876543210"],
  "duplicates": ["5555666677"],
  "total": 2
}
```

**Список заказов пользователя:**

```bash
# С пагинацией (запрос первых 10 заказов)
curl -X GET http://localhost:8080/api/user/orders \
  -b cookies.txt \
  -v
```

**Ответ (200 OK):**

```json
[
  {
    "number": "1234567890",
    "status": "PROCESSED",
    "accrual": 150,
    "uploaded_at": "2024-01-15T10:30:00Z"
  },
  {
    "number": "9876543210",
    "status": "NEW",
    "uploaded_at": "2024-01-15T10:31:00Z"
  }
]
```

### Баланс

**Получение текущего баланса:**

```bash
curl -X GET http://localhost:8080/api/user/balance \
  -b cookies.txt \
  -v
```

**Ответ (200 OK):**

```json
{
  "current": 150.0,
  "withdrawn": 50
}
```

**Списание баллов в счёт оплаты заказа:**

```bash
# Списать 100 баллов в счёт заказа 1234567890
curl -X POST http://localhost:8080/api/user/balance/withdraw \
  -b cookies.txt \
  -H "Content-Type: application/json" \
  -d '{"order": "1234567890", "sum": 100}' \
  -v
```

**Ответ (200 OK):**

```json
{
  "current": 50.0,
  "withdrawn": 150,
  "sum": 100
}
```

**История списаний:**

```bash
curl -X GET http://localhost:8080/api/user/withdrawals \
  -b cookies.txt \
  -v
```

**Ответ (200 OK):**

```json
[
  {
    "order": "1234567890",
    "sum": 100,
    "processed_at": "2024-01-15T11:00:00Z"
  }
]
```

### Расчёт начислений (без аутентификации)

**Получение информации о расчёте начислений:**

```bash
curl -X GET http://localhost:8080/api/orders/1234567890 \
  -v
```

**Ответ (200 OK) — заказ обработан:**

```json
{
  "order": "1234567890",
  "status": "PROCESSED",
  "accrual": 150
}
```

**Ответ (200 OK) — заказ ожидает обработки:**

```json
{
  "order": "9876543210",
  "status": "NEW"
}
```

**Ответ (204 No Content) — заказ не найден:**

```
(empty body)
```

## Примеры ошибок

**Неверный формат запроса (400):**

```bash
curl -X POST http://localhost:8080/api/user/register \
  -H "Content-Type: application/json" \
  -d '{"login": "", "password": "test"}' \
  -v
```

**Ответ (400 Bad Request):**

```json
{
  "error": "login and password are required",
  "code": "VALIDATION_ERROR"
}
```

**Дубликат логина (409):**

```bash
curl -X POST http://localhost:8080/api/user/register \
  -H "Content-Type: application/json" \
  -d '{"login": "ivan.petrov", "password": "AnotherPass123!"}' \
  -v
```

**Ответ (409 Conflict):**

```json
{
  "error": "user with this login already exists",
  "code": "CONFLICT_ERROR"
}
```

**Недостаточно средств (402):**

```bash
curl -X POST http://localhost:8080/api/user/balance/withdraw \
  -b cookies.txt \
  -H "Content-Type: application/json" \
  -d '{"order": "1234567890", "sum": 999999}' \
  -v
```

**Ответ (402 Payment Required):**

```json
{
  "error": "insufficient balance",
  "code": "PAYMENT_ERROR"
}
```

**Ошибка внешнего сервиса (502):**

```bash
# При недоступности PointsCalcSystem
curl -X POST http://localhost:8080/api/user/orders \
  -b cookies.txt \
  -H "Content-Type: application/json" \
  -d '[1234567890]' \
  -v
```

**Ответ (502 Bad Gateway):**

```json
{
  "error": "failed to process orders",
  "code": "GATEWAY_ERROR"
}
```

## Структура проекта

```
.
├── cmd/server/
│   ├── main.go          # Точка входа, инициализация сервера
│   └── routes.go        # Настройка маршрутов (chi router)
├── internal/
│   ├── config/
│   │   ├── config.go        # Конфигурация (env vars + CLI flags)
│   │   └── config_test.go
│   ├── database/
│   │   ├── database.go      # Подключение к PostgreSQL, миграции
│   │   ├── database_test.go
│   │   └── open_test.go
│   ├── models/
│   │   ├── models.go        # Модели данных (User, Order, Transaction)
│   │   └── models_test.go
│   ├── middleware/
│   │   ├── error.go         # Единый формат ошибок, recovery middleware
│   │   └── error_test.go
│   ├── jwt/
│   │   ├── jwt.go           # JWT токены (HMAC-SHA256, bcrypt)
│   │   ├── jwt_test.go
│   │   └── middleware_test.go
│   ├── handler/
│   │   ├── handler.go       # HTTP handlers (auth, orders, balance)
│   │   ├── handler_test.go
│   │   ├── handler_full_test.go
│   │   └── handler_final_test.go
│   └── service/
│       ├── auth_service.go      # Auth, balance, orders, withdrawals
│       ├── auth_service_test.go
│       ├── order_service.go     # Order processing, PointsCalcClient
│       └── order_service_test.go
├── migrations/
│   ├── 000001_create_users.up.sql
│   ├── 000001_create_users.down.sql
│   ├── 000002_create_orders.up.sql
│   ├── 000002_create_orders.down.sql
│   ├── 000003_create_transactions.up.sql
│   └── 000003_create_transactions.down.sql
├── openspec/                # OpenSpec артефакты проекта
│   ├── config.yaml
│   ├── changes/
│   └── specs/
├── go.mod
├── go.sum
└── README.md
```

## API Endpoints

### Аутентификация (публичные)

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/api/user/register` | Регистрация пользователя |
| `POST` | `/api/user/login` | Вход в систему |
| `POST` | `/api/user/refresh` | Обновление JWT-токена |
| `POST` | `/api/user/logout` | Выход из системы |

### Заказы (protected)

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/api/user/orders` | Загрузка номеров заказов |
| `GET` | `/api/user/orders` | Список заказов пользователя |

### Баланс (protected)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/api/user/balance` | Текущий баланс |
| `POST` | `/api/user/balance/withdraw` | Списания баллов |
| `GET` | `/api/user/withdrawals` | История списаний |

### Расчёт начислений (public)

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/api/orders/{number}` | Информация о расчёте начислений |

### Формат ответов

**Успешный ответ (200 OK):**

```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "login": "username"
}
```

**Ошибка (стандартизированный JSON):**

```json
{
  "error": "человеко-читаемое описание ошибки",
  "code": "ERROR_CODE"
}
```

### Коды ошибок

| Код | HTTP статус | Описание |
|---|---|---|
| `VALIDATION_ERROR` | 400, 422 | Неверный формат запроса или полей |
| `AUTHENTICATION_ERROR` | 401 | Пользователь не аутентифицирован |
| `CONFLICT_ERROR` | 409 | Нарушение уникальности (дубликат) |
| `PAYMENT_ERROR` | 402 | Недостаточно средств на балансе |
| `NOT_FOUND_ERROR` | 404 | Ресурс не найден |
| `RATE_LIMIT_ERROR` | 429 | Превышен лимит запросов |
| `SERVER_ERROR` | 500 | Внутренняя ошибка сервера |
| `GATEWAY_ERROR` | 502 | Ошибка внешнего сервиса |

## База данных

### Схема данных

**users:**

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PRIMARY KEY DEFAULT gen_random_uuid() |
| `login` | text | UNIQUE NOT NULL |
| `password_hash` | text | NOT NULL (bcrypt-хеш) |
| `created_at` | timestamptz | NOT NULL DEFAULT now() |

**orders:**

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PRIMARY KEY DEFAULT gen_random_uuid() |
| `user_id` | uuid | NOT NULL, FK → users.id ON DELETE CASCADE |
| `order_number` | text | UNIQUE NOT NULL |
| `status` | text | NOT NULL, DEFAULT NEW (NEW/PROCESSING/INVALID/PROCESSED) |
| `accrual` | int | DEFAULT NULL |
| `uploaded_at` | timestamptz | NOT NULL DEFAULT now() |
| `updated_at` | timestamptz | NOT NULL DEFAULT now() |

Индексы: `orders(user_id, uploaded_at DESC)`, `orders(order_number) UNIQUE`

**transactions:**

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PRIMARY KEY DEFAULT gen_random_uuid() |
| `user_id` | uuid | NOT NULL, FK → users.id ON DELETE CASCADE |
| `order_id` | uuid | NULL, FK → orders.id ON DELETE SET NULL |
| `type` | text | NOT NULL (earned/redeemed) |
| `points` | int | NOT NULL (positive для earned, negative для redeemed) |
| `order_discount` | decimal | NULL |
| `description` | text | NULL |
| `created_at` | timestamptz | NOT NULL DEFAULT now() |

Индекс: `transactions(user_id, created_at DESC)`

## Статусы заказов

| Статус | Описание |
|---|---|
| `NEW` | Заказ принят, ожидает проверки |
| `PROCESSING` | Заказ обрабатывается |
| `INVALID` | Заказ не принят к расчёту (окончательный) |
| `PROCESSED` | Расчёт завершён, баллы начислены (окончательный) |

## Интеграция с PointsCalcSystem

Внешний сервис для расчёта баллов лояльности:

- **Endpoint:** `POST {ACCRUAL_SYSTEM_ADDRESS}/check`
- **Тело запроса:** `{"order_number": "string"}`
- **Ответ:** `{"points": integer}`
- **Таймаут:** 5 секунд
- **Повторы:** 2 попытки при 5xx ошибках

## Безопасность

- JWT-токены (HMAC-SHA256), TTL 1 час
- Пароли: bcrypt с cost 10
- Передача токенов: HTTP-only cookie или `Authorization: Bearer`
- Endpoints для работы с заказами и балансом доступны только аутентифицированным пользователям

## Разработка

### Линтер

```bash
golangci-lint run ./...
```

### Тесты

```bash
# Все тесты
go test ./...

# Покрытие
go test -cover ./...

# Детальное покрытие по файлам
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

**Покрытие тестами:** 60%+

### Миграции

Миграции управляются через `golang-migrate/migrate/v4`. Файлы хранятся в `migrations/`.

```bash
# Применяются автоматически при старте сервера
# Убедитесь, что DATABASE_URI указывает на нужную базу
go run ./cmd/server/ -d "postgres://user:pass@localhost:5432/db"
```

## Технологии

| Назначение | Библиотека |
|---|---|
| HTTP роутер | `github.com/go-chi/chi/v5` |
| JWT | `github.com/golang-jwt/jwt/v5` |
| PostgreSQL | `github.com/lib/pq` |
| Миграции | `github.com/golang-migrate/migrate/v4` |
| Логирование | `github.com/sirupsen/logrus` |
| Пароли | `golang.org/x/crypto/bcrypt` |

## Тестовое покрытие

| Пакет | Покрытие |
|---|---|
| `internal/jwt` | 91.5% |
| `internal/middleware` | 100.0% |
| `internal/handler` | 74.9% |
| `internal/config` | 76.5% |
| `internal/service` | 57.9% |
| `internal/database` | 41.2% |
| **Итого** | **60.3%** |
