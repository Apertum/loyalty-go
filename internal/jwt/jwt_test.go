package jwt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	token, err := GenerateJWT("user-123", "testuser")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestParseToken(t *testing.T) {
	token, err := GenerateJWT("user-123", "testuser")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatalf("expected no error parsing, got %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected user_id user-123, got %s", claims.UserID)
	}
	if claims.UserName != "testuser" {
		t.Errorf("expected user_name testuser, got %s", claims.UserName)
	}
}

func TestParseInvalidToken(t *testing.T) {
	_, err := ParseJWT("invalid-token-string")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestExtractTokenFromHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer my-jwt-token")

	token, err := ExtractToken(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token != "my-jwt-token" {
		t.Errorf("expected token my-jwt-token, got %s", token)
	}
}

func TestExtractTokenFromCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "cookie-token"})

	token, err := ExtractToken(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token != "cookie-token" {
		t.Errorf("expected token cookie-token, got %s", token)
	}
}

func TestExtractTokenMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := ExtractToken(req)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestContextWithUser(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithUser(ctx, "user-456", "john")
	id, ok := UserIDFromContext(ctx)
	if !ok || id != "user-456" {
		t.Errorf("expected user-456, got %s (ok=%v)", id, ok)
	}
}

func TestContextWithUserName(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithUserName(ctx, "jane")
	name, ok := UserNameFromContext(ctx)
	if !ok || name != "jane" {
		t.Errorf("expected jane, got %s (ok=%v)", name, ok)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, 200, map[string]interface{}{"key": "value"})

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}
}

func TestSetAndClearAuthCookie(t *testing.T) {
	w := httptest.NewRecorder()
	SetAuthCookie(w, "test-token")

	cookie := w.Result().Cookies()
	if len(cookie) == 0 {
		t.Fatal("expected cookie to be set")
	}
	if cookie[0].Name != "token" {
		t.Errorf("expected cookie name token, got %s", cookie[0].Name)
	}
	if cookie[0].HttpOnly != true {
		t.Error("expected cookie to be HttpOnly")
	}

	w2 := httptest.NewRecorder()
	ClearAuthCookie(w2)
	cookie2 := w2.Result().Cookies()
	if len(cookie2) == 0 {
		t.Fatal("expected clear cookie")
	}
	if cookie2[0].MaxAge != -1 {
		t.Error("expected MaxAge to be -1")
	}
}

func TestGenerateJWT(t *testing.T) {
	token, err := GenerateJWT("user-789", "alice")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatalf("expected no error parsing, got %v", err)
	}
	if claims.UserID != "user-789" {
		t.Errorf("expected user-789, got %s", claims.UserID)
	}
}

func TestSetSigningKey_CustomKey(t *testing.T) {
	// Сохраняем оригинальный ключ
	originalKey := signingKey
	defer func() { signingKey = originalKey }()

	// Устанавливаем пользовательский ключ
	SetSigningKey("custom-test-key-123")

	// Генерируем и парсим токен с новым ключом
	token, err := GenerateJWT("user-999", "bob")
	if err != nil {
		t.Fatalf("expected no error with custom key, got %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatalf("expected no error parsing with custom key, got %v", err)
	}
	if claims.UserID != "user-999" {
		t.Errorf("expected user-999, got %s", claims.UserID)
	}
}

func TestSetSigningKey_EmptyKey(t *testing.T) {
	originalKey := signingKey
	defer func() { signingKey = originalKey }()

	// Пустой ключ не должен изменять ключ по умолчанию
	SetSigningKey("")
	if signingKey != defaultSigningKey {
		t.Errorf("expected default key, got %s", signingKey)
	}
}
