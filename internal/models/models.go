// Package models определяет все типы сущностей базы данных, используемые в системе лояльности.
package models

import (
	"time"
)

// OrderStatus представляет статус заказа в системе лояльности.
type OrderStatus string

const (
	OrderStatusNew        OrderStatus = "NEW"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusInvalid    OrderStatus = "INVALID"
	OrderStatusProcessed  OrderStatus = "PROCESSED"
)

// TransactionType представляет тип транзакции с баллами лояльности.
type TransactionType string

const (
	TransactionTypeEarned   TransactionType = "earned"
	TransactionTypeRedeemed TransactionType = "redeemed"
)

// User представляет зарегистрированного пользователя в системе лояльности.
type User struct {
	ID           string    `db:"id" json:"id"`
	Login        string    `db:"login" json:"login"`
	PasswordHash string    `db:"password_hash" json:"-"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

// Order представляет заказ клиента, отправленный для расчёта баллов.
type Order struct {
	ID          string      `db:"id" json:"id"`
	UserID      string      `db:"user_id" json:"user_id"`
	OrderNumber string      `db:"order_number" json:"order_number"`
	Status      OrderStatus `db:"status" json:"status"`
	Accrual     *int        `db:"accrual" json:"accrual"`
	UploadedAt  time.Time   `db:"uploaded_at" json:"uploaded_at"`
	UpdatedAt   time.Time   `db:"updated_at" json:"updated_at"`
}

// Transaction представляет транзакцию с баллами лояльности (начисление или списание).
type Transaction struct {
	ID            string    `db:"id" json:"id"`
	UserID        string    `db:"user_id" json:"user_id"`
	OrderID       *string   `db:"order_id" json:"order_id"`
	OrderNumber   string    `db:"order_number" json:"order_number"`
	Type          string    `db:"type" json:"type"`
	Points        int       `db:"points" json:"points"`
	OrderDiscount *float64  `db:"order_discount" json:"order_discount"`
	Description   *string   `db:"description" json:"description"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// BalanceResponse содержит информацию о текущем балансе лояльности пользователя.
type BalanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn int     `json:"withdrawn"`
}
