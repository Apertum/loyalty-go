// Package jwt предоставляет генерацию JWT-токенов, валидацию и middleware для HTTP-аутентификации.
package jwt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// defaultSigningKey — ключ по умолчанию (только для тестов).
	defaultSigningKey = "loyalty-service-secret-key-2024"

	// TokenTTL — время жизни для токенов доступа и обновления.
	TokenTTL = 1 * time.Hour

	// AuthorizationHeader — имя HTTP-заголовка для bearer-токенов.
	AuthorizationHeader = "Authorization"

	// BearerScheme — префикс для bearer-токенов в заголовке Authorization.
	BearerScheme = "Bearer "

	// cookieName — имя HTTP-only cookie для хранения токена.
	cookieName = "token"

	// contextKeyUserID — ключ контекста для user ID.
	contextKeyUserID contextKey = "userID"

	// contextKeyUserName — ключ контекста для имени пользователя (login).
	contextKeyUserName contextKey = "userName"
)

type contextKey string

var (
	// signingKey — секретный ключ для подписи JWT. Устанавливается через SetSigningKey.
	signingKey = defaultSigningKey
)

// SetSigningKey устанавливает секретный ключ для подписи JWT.
// Должен вызываться при запуске приложения из конфигурации.
func SetSigningKey(key string) {
	if key != "" {
		signingKey = key
	}
}

var (
	// ErrTokenExpired возвращается, когда JWT-токен истёк.
	ErrTokenExpired = errors.New("token has expired")

	// ErrInvalidToken возвращается, когда JWT-токен недействителен.
	ErrInvalidToken = errors.New("invalid token")

	// ErrMissingToken возвращается, когда отсутствует заголовок Authorization или cookie.
	ErrMissingToken = errors.New("missing token")
)

// Claims определяет структуру payload JWT-токена.
type Claims struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	jwt.RegisteredClaims
}

// GenerateToken создаёт новый JWT-токен для указанного пользователя.
func GenerateToken(userID, userName string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		UserName: userName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(signingKey))
}

// ParseToken валидирует и разбирает строку JWT-токена, возвращая claims.
func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(signingKey), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ExtractToken извлекает JWT-токен из заголовка Authorization или cookie.
func ExtractToken(r *http.Request) (string, error) {
	// Сначала пробуем заголовок Authorization
	auth := r.Header.Get(AuthorizationHeader)
	if auth != "" && strings.HasPrefix(auth, BearerScheme) {
		return strings.TrimPrefix(auth, BearerScheme), nil
	}

	// Пробуем cookie
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return "", ErrMissingToken
	}

	return cookie.Value, nil
}

// ContextWithUser добавляет информацию о пользователе в контекст запроса.
func ContextWithUser(ctx context.Context, userID, userName string) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// ContextWithUserName добавляет имя пользователя в контекст запроса.
func ContextWithUserName(ctx context.Context, userName string) context.Context {
	return context.WithValue(ctx, contextKeyUserName, userName)
}

// UserIDFromContext извлекает user ID из контекста запроса.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKeyUserID).(string)
	return id, ok
}

// UserNameFromContext извлекает имя пользователя из контекста запроса.
func UserNameFromContext(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(contextKeyUserName).(string)
	return name, ok
}

// AuthMiddleware создаёт HTTP middleware, которое валидирует JWT-токены и добавляет контекст пользователя.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := ExtractToken(r)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"error": "authentication required",
				"code":  "AUTHENTICATION_ERROR",
			})
			return
		}

		claims, err := ParseToken(tokenString)
		if err != nil {
			WriteJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"error": "invalid token",
				"code":  "AUTHENTICATION_ERROR",
			})
			return
		}

		ctx := ContextWithUser(r.Context(), claims.UserID, claims.UserName)
		ctx = ContextWithUserName(ctx, claims.UserName)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ParseJWT разбирает строку JWT-токена и возвращает claims без HTTP-обработки.
// Полезно для handler'ов login и registration.
func ParseJWT(tokenString string) (*Claims, error) {
	return ParseToken(tokenString)
}

// GenerateJWT создаёт новую строку JWT-токена для указанного пользователя.
// Полезно для handler'ов login и registration.
func GenerateJWT(userID, userName string) (string, error) {
	return GenerateToken(userID, userName)
}

// SetAuthCookie устанавливает JWT-токен в виде HTTP-only cookie.
func SetAuthCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(TokenTTL.Seconds()),
	}
	http.SetCookie(w, cookie)
}

// ClearAuthCookie удаляет cookie аутентификации.
func ClearAuthCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	}
	http.SetCookie(w, cookie)
}

// WriteJSON записывает JSON-ответ с указанным кодом статуса.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
