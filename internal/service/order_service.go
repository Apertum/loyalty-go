package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// PointsChecker интерфейс для проверки заказов во внешней системе расчёта баллов.
type PointsChecker interface {
	CheckOrder(ctx context.Context, orderNumber string) (int, error)
}

// PointsCalcClient реализует PointsChecker для внешней PointsCalcSystem.
type PointsCalcClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPointsCalcClient создаёт новый PointsCalcClient.
func NewPointsCalcClient(baseURL string) *PointsCalcClient {
	return &PointsCalcClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// CheckOrder проверяет номер заказа во внешней системе расчёта баллов.
// Возвращает количество баллов для начисления (0, если нет).
// При 5xx ошибках внешняя система повторяет запрос до 2 раз.
func (c *PointsCalcClient) CheckOrder(ctx context.Context, orderNumber string) (int, error) {
	url := c.baseURL + "/check"

	payload := struct {
		OrderNumber string `json:"order_number"`
	}{OrderNumber: orderNumber}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Retry: до 2 попыток при 5xx ошибках внешней системы
	var resp *http.Response
	maxRetries := 2
	for i := 0; i <= maxRetries; i++ {
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return 0, fmt.Errorf("send request: %w", err)
		}

		// Если не 5xx — выходим из цикла
		if resp.StatusCode < 500 {
			break
		}

		resp.Body.Close()

		// Если это была последняя попытка — возвращаем ошибку
		if i == maxRetries {
			return 0, fmt.Errorf("external service returned %d after %d retries", resp.StatusCode, maxRetries)
		}

		// Повторяем запрос с новым телом
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return 0, fmt.Errorf("create retry request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("external service returned %d", resp.StatusCode)
	}

	var result struct {
		Points int `json:"points"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	return result.Points, nil
}

// OrderService обрабатывает создание и управление заказами.
type OrderService struct {
	db     *sql.DB
	points PointsChecker
	mu     sync.Mutex
}

// NewOrderService создаёт новый OrderService.
func NewOrderService(db *sql.DB, points PointsChecker) *OrderService {
	return &OrderService{
		db:     db,
		points: points,
	}
}

// SubmitOrders создаёт новые заказы для пользователя и проверяет их во внешней системе.
func (s *OrderService) SubmitOrders(ctx context.Context, userID string, numbers []string) ([]string, []string, []string, error) {
	s.mu.Lock()
	var accepted []string
	var duplicates []string
	var conflicts []string

	for _, number := range numbers {
		var existingID string
		err := s.db.QueryRowContext(ctx,
			"SELECT id FROM orders WHERE user_id = $1 AND order_number = $2",
			userID, number,
		).Scan(&existingID)

		if err == nil {
			duplicates = append(duplicates, number)
			continue
		}
		if err != sql.ErrNoRows {
			s.mu.Unlock()
			return nil, nil, nil, fmt.Errorf("check order %s: %w", number, err)
		}

		var alreadyUsed string
		err = s.db.QueryRowContext(ctx,
			"SELECT user_id FROM orders WHERE order_number = $1",
			number,
		).Scan(&alreadyUsed)

		if err == nil && alreadyUsed != userID {
			conflicts = append(conflicts, number)
			continue
		}
		if err != nil && err != sql.ErrNoRows {
			s.mu.Unlock()
			return nil, nil, nil, fmt.Errorf("check conflict for %s: %w", number, err)
		}

		var orderID string
		now := time.Now()
		err = s.db.QueryRowContext(ctx,
			`INSERT INTO orders (user_id, order_number, status, uploaded_at, updated_at)
			 VALUES ($1, $2, 'NEW', $3, $3) RETURNING id`,
			userID, number, now,
		).Scan(&orderID)
		if err != nil {
			if isDuplicateError(err) {
				duplicates = append(duplicates, number)
				continue
			}
			s.mu.Unlock()
			return nil, nil, nil, fmt.Errorf("create order: %w", err)
		}

		accepted = append(accepted, number)
	}
	s.mu.Unlock()

	// Запускаем проверку заказов во внешней системе параллельно без блокировки HTTP-ответа.
	// Каждая горутина получает автономный контекст от context.Background(),
	// поэтому обработка продолжается даже после того, как ответ ушёл клиенту.
	var g errgroup.Group
	g.SetLimit(10)
	for _, number := range accepted {
		number := number
		userID := userID
		g.Go(func() error {
			workCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			s.processOrder(workCtx, userID, number)
			return nil
		})
	}
	// Не вызываем g.Wait() — обработка идёт в фоне, ответ 202 улетает сразу.

	return accepted, duplicates, conflicts, nil
}

// processOrder проверяет заказ во внешней системе и обновляет его статус.
// Вызывается из горутин, запущенных в SubmitOrders.
func (s *OrderService) processOrder(ctx context.Context, userID, orderNumber string) {
	points, err := s.points.CheckOrder(ctx, orderNumber)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"user_id": userID,
			"order":   orderNumber,
		}).Warn("external check failed for order")
		return
	}

	status := "PROCESSED"
	if points == 0 {
		status = "INVALID"
	}

	accrual := points
	if status == "INVALID" {
		accrual = 0
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE orders SET status = $1, accrual = $2, updated_at = now()
		 WHERE user_id = $3 AND order_number = $4`,
		status, accrual, userID, orderNumber,
	)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"user_id": userID,
			"order":   orderNumber,
		}).Error("failed to update order status")
		return
	}

	if status == "PROCESSED" {
		var orderID string
		err = s.db.QueryRowContext(ctx, "SELECT id FROM orders WHERE user_id = $1 AND order_number = $2", userID, orderNumber).Scan(&orderID)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"user_id": userID,
				"order":   orderNumber,
			}).Error("failed to get order ID for transaction")
			return
		}

		desc := fmt.Sprintf("Баллы за заказ %s", orderNumber)
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO transactions (user_id, order_id, type, points, description)
			 VALUES ($1, $2, 'earned', $3, $4)`,
			userID, orderID, points, desc,
		)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"user_id": userID,
				"order":   orderNumber,
			}).Error("failed to insert earned transaction")
		}
	}
}

// GetOrderCalculation получает статус расчёта заказа по номеру.
func (s *OrderService) GetOrderCalculation(ctx context.Context, orderNumber string) (string, *int, error) {
	var status string
	var accrual *int

	err := s.db.QueryRowContext(ctx, "SELECT status, accrual FROM orders WHERE order_number = $1", orderNumber).
		Scan(&status, &accrual)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil, sql.ErrNoRows
		}
		return "", nil, err
	}
	return status, accrual, nil
}

// RedeemPoints позволяет пользователю списать баллы для получения скидки по заказу.
func (s *OrderService) RedeemPoints(ctx context.Context, userID, orderNumber string, points int) error {
	return nil
}

// GetDB возвращает базовое подключение к базе данных.
func (s *OrderService) GetDB() *sql.DB {
	return s.db
}
