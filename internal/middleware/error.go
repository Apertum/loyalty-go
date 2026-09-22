// Package middleware предоставляет HTTP middleware для обработки ошибок, аутентификации и обработки запросов.
package middleware

import (
	"encoding/json"
	"net/http"
)

// ErrorCode представляет машинно-читаемый код ошибки, возвращаемый API.
type ErrorCode string

const (
	ValidationError     ErrorCode = "VALIDATION_ERROR"
	AuthenticationError ErrorCode = "AUTHENTICATION_ERROR"
	ConflictError       ErrorCode = "CONFLICT_ERROR"
	PaymentError        ErrorCode = "PAYMENT_ERROR"
	NotFoundError       ErrorCode = "NOT_FOUND_ERROR"
	RateLimitError      ErrorCode = "RATE_LIMIT_ERROR"
	ServerError         ErrorCode = "SERVER_ERROR"
	GatewayError        ErrorCode = "GATEWAY_ERROR"
)

// APIError представляет стандартизированный ответ об ошибке API.
type APIError struct {
	Error string    `json:"error"`
	Code  ErrorCode `json:"code"`
}

// WriteError записывает стандартизированный JSON-ответ об ошибке с указанным кодом HTTP-статуса.
func WriteError(w http.ResponseWriter, statusCode int, code ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIError{
		Error: message,
		Code:  code,
	})
}

// RecoveryMiddleware восстанавливается после паники и возвращает 500 Internal Server Error.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				WriteError(w, http.StatusInternalServerError, ServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
