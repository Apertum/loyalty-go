// Package service предоставляет бизнес-логику для системы лояльности.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"loyalty-service/internal/jwt"
	"loyalty-service/internal/models"

	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidCredentials возвращается, когда логин или пароль неверны.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserExists возвращается, когда пользователь с таким логином уже существует.
	ErrUserExists = errors.New("user with this login already exists")
	// ErrInvalidToken возвращается, когда токен недействителен или истёк.
	ErrInvalidToken = errors.New("invalid token")
)

// AuthService обрабатывает регистрацию пользователей, аутентификацию и управление токенами.
type AuthService struct {
	db *sql.DB
}

// NewAuthService создаёт новый экземпляр AuthService.
func NewAuthService(db *sql.DB) *AuthService {
	return &AuthService{db: db}
}

// Register создаёт новый пользовательский аккаунт и возвращает JWT-токен.
func (s *AuthService) Register(ctx context.Context, login, password string) (*jwt.Claims, error) {
	if login == "" || password == "" {
		return nil, fmt.Errorf("empty login or password")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var userID string
	err = s.db.QueryRowContext(ctx,
		"INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id",
		login, string(hash),
	).Scan(&userID)
	if err != nil {
		if isDuplicateError(err) {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	token, err := jwt.GenerateJWT(userID, login)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	claims, err := jwt.ParseJWT(token)
	if err != nil {
		return nil, fmt.Errorf("parse generated token: %w", err)
	}

	return claims, nil
}

// Login аутентифицирует пользователя и возвращает JWT-токен.
func (s *AuthService) Login(ctx context.Context, login, password string) (*jwt.Claims, error) {
	var user models.User
	err := s.db.QueryRowContext(ctx,
		"SELECT id, login, password_hash FROM users WHERE login = $1",
		login,
	).Scan(&user.ID, &user.Login, &user.PasswordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("query user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := jwt.GenerateJWT(user.ID, user.Login)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	claims, err := jwt.ParseJWT(token)
	if err != nil {
		return nil, fmt.Errorf("parse generated token: %w", err)
	}

	return claims, nil
}

// Logout отменяет действие указанного токена, отзывая его.
func (s *AuthService) Logout(ctx context.Context, token string) error {
	// Для stateless JWT мы полагаемся на TTL.
	// В более сложной системе можно поддерживать чёрный список токенов.
	return nil
}

// RefreshToken валидирует существующий токен и выдаёт новый.
func (s *AuthService) RefreshToken(ctx context.Context, token string) (*jwt.Claims, error) {
	claims, err := jwt.ParseJWT(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	newToken, err := jwt.GenerateJWT(claims.UserID, claims.UserName)
	if err != nil {
		return nil, fmt.Errorf("generate new token: %w", err)
	}

	return jwt.ParseJWT(newToken)
}

// GetDB возвращает базовое подключение к базе данных.
func (s *AuthService) GetDB() *sql.DB {
	return s.db
}

// isDuplicateError проверяет, является ли ошибка базы данных нарушением уникальности.
// Код ошибки PostgreSQL 23505 = unique_violation.
func isDuplicateError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// GetBalance получает текущий баланс лояльности пользователя.
func (s *AuthService) GetBalance(ctx context.Context, userID string) (*models.BalanceResponse, error) {
	var currentBalance sql.NullFloat64
	var totalWithdrawn sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN t.points > 0 THEN t.points ELSE 0 END), 0)::float,
			COALESCE(SUM(ABS(CASE WHEN t.points < 0 THEN t.points ELSE 0 END)), 0)
		FROM transactions t
		WHERE t.user_id = $1
	`, userID).Scan(&currentBalance, &totalWithdrawn)
	if err != nil {
		return nil, fmt.Errorf("query balance: %w", err)
	}

	return &models.BalanceResponse{
		Current:   currentBalance.Float64,
		Withdrawn: int(totalWithdrawn.Int64),
	}, nil
}

// GetOrder получает заказ по его номеру для указанного пользователя.
func (s *AuthService) GetOrder(ctx context.Context, userID, orderNumber string) (*models.Order, error) {
	var o models.Order
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, order_number, status, accrual, uploaded_at, updated_at
		FROM orders
		WHERE user_id = $1 AND order_number = $2
	`, userID, orderNumber).Scan(
		&o.ID, &o.UserID, &o.OrderNumber, &o.Status, &o.Accrual, &o.UploadedAt, &o.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order not found")
		}
		return nil, fmt.Errorf("query order: %w", err)
	}
	return &o, nil
}

// GetWithdrawals получает историю списаний для указанного пользователя.
func (s *AuthService) GetWithdrawals(ctx context.Context, userID string) ([]models.Transaction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.user_id, t.order_id, o.order_number, t.type, t.points, t.order_discount, t.description, t.created_at
		FROM transactions t
		LEFT JOIN orders o ON t.order_id = o.id
		WHERE t.user_id = $1 AND t.type = 'redeemed'
		ORDER BY t.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []models.Transaction
	for rows.Next() {
		var t models.Transaction
		var orderDiscount sql.NullFloat64
		err := rows.Scan(&t.ID, &t.UserID, &t.OrderID, &t.OrderNumber, &t.Type, &t.Points, &orderDiscount, &t.Description, &t.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan withdrawal: %w", err)
		}
		if orderDiscount.Valid {
			t.OrderDiscount = &orderDiscount.Float64
		}
		withdrawals = append(withdrawals, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate withdrawals: %w", err)
	}

	return withdrawals, nil
}

// GetOrders получает список заказов для указанного пользователя.
func (s *AuthService) GetOrders(ctx context.Context, userID string) ([]models.Order, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, order_number, status, accrual, uploaded_at, updated_at
		FROM orders
		WHERE user_id = $1
		ORDER BY uploaded_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var o models.Order
		err := rows.Scan(&o.ID, &o.UserID, &o.OrderNumber, &o.Status, &o.Accrual, &o.UploadedAt, &o.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}

	return orders, nil
}
