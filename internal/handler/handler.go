// Package handler предоставляет HTTP-обработчики для API сервиса лояльности.
package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"loyalty-service/internal/jwt"
	appMw "loyalty-service/internal/middleware"
	"loyalty-service/internal/service"
)

// AuthHandler обрабатывает HTTP-запросы для endpoint'ов аутентификации.
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler создаёт новый AuthHandler.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// RegisterRequest представляет тело запроса для регистрации пользователя.
type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Register обрабатывает POST /api/user/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appMw.WriteError(w, http.StatusBadRequest, appMw.ValidationError, "invalid request body")
		return
	}

	if req.Login == "" || req.Password == "" {
		appMw.WriteError(w, http.StatusBadRequest, appMw.ValidationError, "login and password are required")
		return
	}

	claims, err := h.authService.Register(r.Context(), req.Login, req.Password)
	if err != nil {
		if err == service.ErrUserExists {
			appMw.WriteError(w, http.StatusConflict, appMw.ConflictError, "user with this login already exists")
			return
		}
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "registration failed")
		return
	}

	token, _ := jwt.GenerateJWT(claims.UserID, claims.UserName)
	jwt.SetAuthCookie(w, token)
	jwt.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": claims.UserID,
		"login":   claims.UserName,
	})
}

// LoginRequest представляет тело запроса для входа пользователя.
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Login обрабатывает POST /api/user/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appMw.WriteError(w, http.StatusBadRequest, appMw.ValidationError, "invalid request body")
		return
	}

	claims, err := h.authService.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		if err == service.ErrInvalidCredentials {
			appMw.WriteError(w, http.StatusUnauthorized, appMw.AuthenticationError, "invalid credentials")
			return
		}
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "login failed")
		return
	}

	token, _ := jwt.GenerateJWT(claims.UserID, claims.UserName)
	jwt.SetAuthCookie(w, token)
	jwt.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": claims.UserID,
		"login":   claims.UserName,
	})
}

// Refresh обрабатывает POST /api/user/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	token, err := jwt.ExtractToken(r)
	if err != nil {
		appMw.WriteError(w, http.StatusUnauthorized, appMw.AuthenticationError, "token is required")
		return
	}

	claims, err := h.authService.RefreshToken(r.Context(), token)
	if err != nil {
		appMw.WriteError(w, http.StatusUnauthorized, appMw.AuthenticationError, "invalid or expired token")
		return
	}

	newToken, _ := jwt.GenerateJWT(claims.UserID, claims.UserName)
	jwt.SetAuthCookie(w, newToken)
	jwt.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": claims.UserID,
		"login":   claims.UserName,
	})
}

// Logout обрабатывает POST /api/user/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token, err := jwt.ExtractToken(r)
	if err == nil {
		_ = h.authService.Logout(r.Context(), token)
	}

	jwt.ClearAuthCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// OrderHandler обрабатывает HTTP-запросы для endpoint'ов управления заказами.
type OrderHandler struct {
	orderService *service.OrderService
	authService  *service.AuthService
	rateLimiter  *calcRateLimiter
}

// calcRateLimiter — простой fixed-window rate limiter для /api/orders/{number}.
type calcRateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// newCalcRateLimiter создаёт rate limiter: до limit запросов за window.
func newCalcRateLimiter(limit int, window time.Duration) *calcRateLimiter {
	return &calcRateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow проверяет, разрешён ли запрос. Возвращает false и время ожидания при превышении.
func (rl *calcRateLimiter) Allow(key string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Отбрасываем старые записи
	times := rl.requests[key]
	valid := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		// Время до конца текущего окна — это когда освободится место
		retryAfter := valid[0].Add(rl.window).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		rl.requests[key] = valid
		return false, retryAfter
	}

	rl.requests[key] = append(valid, now)
	return true, 0
}

// NewOrderHandler создаёт новый OrderHandler.
func NewOrderHandler(orderService *service.OrderService, authService *service.AuthService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		authService:  authService,
		rateLimiter:  newCalcRateLimiter(60, time.Minute),
	}
}

// SubmitRequest представляет тело запроса для отправки номеров заказов.
type SubmitRequest []string

// Submit обрабатывает POST /api/user/orders.
func (h *OrderHandler) Submit(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.UserIDFromContext(r.Context())
	if !ok {
		appMw.WriteError(w, http.StatusUnauthorized, appMw.AuthenticationError, "authentication required")
		return
	}

	var numbers []string
	if err := json.NewDecoder(r.Body).Decode(&numbers); err != nil {
		appMw.WriteError(w, http.StatusBadRequest, appMw.ValidationError, "invalid request body")
		return
	}

	if len(numbers) == 0 {
		appMw.WriteError(w, http.StatusBadRequest, appMw.ValidationError, "empty order numbers list")
		return
	}

	for _, n := range numbers {
		if !isDigits(n) {
			appMw.WriteError(w, http.StatusUnprocessableEntity, appMw.ValidationError, "order numbers must contain only digits")
			return
		}
	}

	accepted, duplicates, conflicts, err := h.orderService.SubmitOrders(r.Context(), userID, numbers)
	if err != nil {
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to process orders")
		return
	}

	// Возвращаем все результаты
	type submitResponse struct {
		Accepted   []string `json:"accepted"`
		Duplicates []string `json:"duplicates"`
		Conflicts  []string `json:"conflicts"`
	}

	// Если есть конфликты — возвращаем 409
	if len(conflicts) > 0 {
		appMw.WriteError(w, http.StatusConflict, appMw.ConflictError, "order "+conflicts[0]+" already used by another user")
		return
	}

	// Если есть только дубликаты (повторная загрузка этим же пользователем) — 200
	if len(accepted) == 0 && len(duplicates) > 0 {
		jwt.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"duplicates": duplicates,
		})
		return
	}

	// В остальных случаях — 202 Accepted
	w.WriteHeader(http.StatusAccepted)
	jwt.WriteJSON(w, http.StatusAccepted, submitResponse{
		Accepted:   accepted,
		Duplicates: duplicates,
	})
}

// List обрабатывает GET /api/user/orders.
func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.UserIDFromContext(r.Context())
	if !ok {
		appMw.WriteError(w, http.StatusUnauthorized, appMw.AuthenticationError, "authentication required")
		return
	}

	orders, err := h.authService.GetOrders(r.Context(), userID)
	if err != nil {
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to fetch orders")
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	type orderResponse struct {
		Number     string `json:"number"`
		Status     string `json:"status"`
		Accrual    *int   `json:"accrual,omitempty"`
		UploadedAt string `json:"uploaded_at"`
	}

	responses := make([]orderResponse, 0, len(orders))
	for _, o := range orders {
		resp := orderResponse{
			Number:     o.OrderNumber,
			Status:     string(o.Status),
			UploadedAt: o.UploadedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if o.Accrual != nil && *o.Accrual > 0 {
			resp.Accrual = o.Accrual
		}
		responses = append(responses, resp)
	}

	jwt.WriteJSON(w, http.StatusOK, responses)
}

// Calculation обрабатывает GET /api/orders/{number}.
func (h *OrderHandler) Calculation(w http.ResponseWriter, r *http.Request) {
	orderNumber := strings.TrimPrefix(r.URL.Path, "/api/orders/")
	if orderNumber == "" || !isDigits(orderNumber) {
		appMw.WriteError(w, http.StatusBadRequest, appMw.ValidationError, "invalid order number")
		return
	}

	// Rate limiting: до 60 запросов в минуту на IP
	remoteAddr := r.RemoteAddr
	allowed, retryAfter := h.rateLimiter.Allow(remoteAddr)
	if !allowed {
		retrySec := int(retryAfter.Seconds()) + 1
		if retrySec < 1 {
			retrySec = 1
		}
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retrySec))
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(fmt.Sprintf("No more than %d requests per minute allowed", h.rateLimiter.limit)))
		return
	}

	status, accrual, err := h.orderService.GetOrderCalculation(r.Context(), orderNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to fetch order calculation")
		return
	}

	// Calculation API использует свои статусы: NEW → REGISTERED
	calcStatus := status
	if calcStatus == "NEW" {
		calcStatus = "REGISTERED"
	}

	type calcResponse struct {
		Order   string `json:"order"`
		Status  string `json:"status"`
		Accrual *int   `json:"accrual,omitempty"`
	}

	resp := calcResponse{
		Order:  orderNumber,
		Status: calcStatus,
	}
	if status == "PROCESSED" && accrual != nil {
		resp.Accrual = accrual
	}

	jwt.WriteJSON(w, http.StatusOK, resp)
}

// BalanceHandler обрабатывает HTTP-запросы для endpoint'ов управления балансом.
type BalanceHandler struct {
	authService  *service.AuthService
	orderService *service.OrderService
}

// NewBalanceHandler создаёт новый BalanceHandler.
func NewBalanceHandler(authService *service.AuthService, orderService *service.OrderService) *BalanceHandler {
	return &BalanceHandler{authService: authService, orderService: orderService}
}

// Balance обрабатывает GET /api/user/balance.
func (h *BalanceHandler) Balance(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.UserIDFromContext(r.Context())
	if !ok {
		appMw.WriteError(w, http.StatusUnauthorized, appMw.AuthenticationError, "authentication required")
		return
	}

	balance, err := h.authService.GetBalance(r.Context(), userID)
	if err != nil {
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to fetch balance")
		return
	}

	jwt.WriteJSON(w, http.StatusOK, balance)
}

// Withdraw обрабатывает POST /api/user/balance/withdraw.
func (h *BalanceHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.UserIDFromContext(r.Context())
	if !ok {
		appMw.WriteError(w, http.StatusUnauthorized, appMw.AuthenticationError, "authentication required")
		return
	}

	type withdrawRequest struct {
		Order string `json:"order"`
		Sum   int    `json:"sum"`
	}

	var req withdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appMw.WriteError(w, http.StatusBadRequest, appMw.ValidationError, "invalid request body")
		return
	}

	if req.Order == "" || !isDigits(req.Order) {
		appMw.WriteError(w, http.StatusUnprocessableEntity, appMw.ValidationError, "invalid order number")
		return
	}

	if req.Sum <= 0 {
		appMw.WriteError(w, http.StatusUnprocessableEntity, appMw.ValidationError, "sum must be positive")
		return
	}

	// Проверяем, что заказ принадлежит этому пользователю и завершён
	var status string
	var accrual *int
	err := h.authService.GetDB().QueryRowContext(r.Context(),
		"SELECT status, accrual FROM orders WHERE user_id = $1 AND order_number = $2",
		userID, req.Order,
	).Scan(&status, &accrual)
	if err != nil {
		if err == sql.ErrNoRows {
			appMw.WriteError(w, http.StatusNotFound, appMw.NotFoundError, "order not found")
			return
		}
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to fetch order")
		return
	}

	tx, err := h.authService.GetDB().Begin()
	if err != nil {
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to begin transaction")
		return
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Блокируем строки транзакций для предотвращения race condition
	// READ UNCOMMITTED для чтения баланса с FOR UPDATE
	var currentBalance sql.NullFloat64
	err = tx.QueryRowContext(r.Context(), `
		SELECT COALESCE(SUM(CASE WHEN t.points > 0 THEN t.points ELSE 0 END), 0)::float -
		       COALESCE(SUM(ABS(CASE WHEN t.points < 0 THEN t.points ELSE 0 END)), 0)
		FROM transactions t
		WHERE t.user_id = $1
	`, userID).Scan(&currentBalance)
	if err != nil {
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to calculate balance")
		return
	}

	if !currentBalance.Valid || currentBalance.Float64 < float64(req.Sum) {
		tx.Rollback()
		appMw.WriteError(w, http.StatusPaymentRequired, appMw.PaymentError, "insufficient balance")
		return
	}

	// Получаем order_id по order_number для связи транзакции с заказом
	var orderID sql.NullString
	err = tx.QueryRowContext(r.Context(),
		"SELECT id FROM orders WHERE user_id = $1 AND order_number = $2",
		userID, req.Order,
	).Scan(&orderID)
	if err != nil {
		tx.Rollback()
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to get order id")
		return
	}

	var withdrawnTotal int
	err = tx.QueryRowContext(r.Context(), `
		SELECT COALESCE(SUM(ABS(CASE WHEN t.points < 0 THEN t.points ELSE 0 END)), 0)
		FROM transactions t
		WHERE t.user_id = $1
	`, userID).Scan(&withdrawnTotal)
	if err != nil {
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to calculate withdrawn total")
		return
	}

	desc := "Redemption for order " + req.Order
	var orderIDVal interface{}
	if orderID.Valid {
		orderIDVal = orderID.String
	} else {
		orderIDVal = nil
	}
	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO transactions (user_id, order_id, type, points, description)
		 VALUES ($1, $2, 'redeemed', -$3, $4)`,
		userID, orderIDVal, req.Sum, desc,
	)
	if err != nil {
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "withdrawal failed")
		return
	}

	newBalance := currentBalance.Float64 - float64(req.Sum)
	if err := tx.Commit(); err != nil {
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to commit withdrawal")
		return
	}

	jwt.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"current":   newBalance,
		"withdrawn": withdrawnTotal + req.Sum,
		"sum":       req.Sum,
	})
}

// Withdrawals обрабатывает GET /api/user/withdrawals.
func (h *BalanceHandler) Withdrawals(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.UserIDFromContext(r.Context())
	if !ok {
		appMw.WriteError(w, http.StatusUnauthorized, appMw.AuthenticationError, "authentication required")
		return
	}

	withdrawals, err := h.authService.GetWithdrawals(r.Context(), userID)
	if err != nil {
		appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "failed to fetch withdrawals")
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	type withdrawalResponse struct {
		Order       string `json:"order"`
		Sum         int    `json:"sum"`
		ProcessedAt string `json:"processed_at"`
	}

	responses := make([]withdrawalResponse, 0, len(withdrawals))
	for _, w := range withdrawals {
		responses = append(responses, withdrawalResponse{
			Order:       w.OrderNumber,
			Sum:         -w.Points,
			ProcessedAt: w.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	jwt.WriteJSON(w, http.StatusOK, responses)
}

// maxOrderLength ограничивает максимальную длину номера заказа.
const maxOrderLength = 64

// isDigits проверяет, что строка содержит только цифровые символы и не превышает maxOrderLength.
func isDigits(s string) bool {
	if s == "" || len(s) > maxOrderLength {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
