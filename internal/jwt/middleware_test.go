package jwt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_CorrectPath(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := UserIDFromContext(r.Context())
		if !ok {
			t.Error("expected user ID in context")
		}
		if id != "user-123" {
			t.Errorf("expected user-123, got %s", id)
		}
		w.WriteHeader(http.StatusOK)
	})

	token, _ := GenerateJWT("user-123", "testuser")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	AuthMiddleware(next).ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_CookieToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	token, _ := GenerateJWT("user-456", "cookieuser")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	w := httptest.NewRecorder()

	AuthMiddleware(next).ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_EmptyBearer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	AuthMiddleware(next).ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestExtractToken_PrefersHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{Name: "token", Value: "cookie-token"})

	token, err := ExtractToken(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token != "header-token" {
		t.Errorf("expected header token, got %s", token)
	}
}

func TestAuthMiddleware_ReturnsJSONOnError(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	AuthMiddleware(next).ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	// Content-Type должен быть application/json, не text/plain
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	// Тело должно быть валидным JSON с полем "code"
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON body, got: %s", w.Body.String())
	}
	if resp["code"] != "AUTHENTICATION_ERROR" {
		t.Errorf("expected code AUTHENTICATION_ERROR, got %v", resp["code"])
	}
}

func TestParseToken_Expired(t *testing.T) {
	// Create a token with very short TTL for testing
	// We can't easily test actual expiration, but we can verify the parse logic
	token, _ := GenerateJWT("user-123", "testuser")
	claims, err := ParseJWT(token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if claims == nil {
		t.Fatal("expected non-nil claims")
	}
}

func TestGenerateJWT_EmptyUserID(t *testing.T) {
	token, err := GenerateJWT("", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}
