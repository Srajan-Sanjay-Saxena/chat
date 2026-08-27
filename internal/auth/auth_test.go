package auth_test

import (
	"chat-v2/internal/auth"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

const testSecret = "test-secret-key-that-is-at-least-32-bytes-long"

func newTestMaker(t *testing.T) *auth.JWTMaker {
	t.Helper()
	maker, err := auth.NewJWTMaker(testSecret)
	if err != nil {
		t.Fatalf("NewJWTMaker() error: %v", err)
	}
	return maker
}

// --- JWTMaker Tests ---

func TestNewJWTMaker_EmptySecret(t *testing.T) {
	_, err := auth.NewJWTMaker("")
	if err == nil {
		t.Fatal("expected error for empty secret, got nil")
	}
}

func TestCreateToken_Valid(t *testing.T) {
	maker := newTestMaker(t)
	userID := uuid.New()

	token, err := maker.CreateToken(userID, time.Hour)
	if err != nil {
		t.Fatalf("CreateToken() error: %v", err)
	}
	if token == "" {
		t.Fatal("CreateToken() returned empty token")
	}
}

func TestCreateToken_NilUserID(t *testing.T) {
	maker := newTestMaker(t)

	_, err := maker.CreateToken(uuid.Nil, time.Hour)
	if err == nil {
		t.Fatal("expected error for nil userID, got nil")
	}
}

func TestCreateToken_ZeroDuration(t *testing.T) {
	maker := newTestMaker(t)

	_, err := maker.CreateToken(uuid.New(), 0)
	if err == nil {
		t.Fatal("expected error for zero duration, got nil")
	}
}

func TestCreateToken_NegativeDuration(t *testing.T) {
	maker := newTestMaker(t)

	_, err := maker.CreateToken(uuid.New(), -time.Hour)
	if err == nil {
		t.Fatal("expected error for negative duration, got nil")
	}
}

func TestVerifyToken_Valid(t *testing.T) {
	maker := newTestMaker(t)
	userID := uuid.New()

	token, err := maker.CreateToken(userID, time.Hour)
	if err != nil {
		t.Fatalf("CreateToken() error: %v", err)
	}

	claims, err := maker.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken() error: %v", err)
	}

	gotUserID, err := claims.UserID()
	if err != nil {
		t.Fatalf("claims.UserID() error: %v", err)
	}
	if gotUserID != userID {
		t.Errorf("claims.UserID() = %v, want %v", gotUserID, userID)
	}
	if claims.Issuer != "relay" {
		t.Errorf("claims.Issuer = %q, want %q", claims.Issuer, "relay")
	}
	if claims.Subject != userID.String() {
		t.Errorf("claims.Subject = %q, want %q", claims.Subject, userID.String())
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	maker := newTestMaker(t)
	userID := uuid.New()

	// Create token that expired 1 hour ago
	token, err := maker.CreateToken(userID, -time.Second)
	// CreateToken rejects negative duration, so we test with a very short one
	// and then manually verify. Instead, let's test with empty token.
	_ = token
	_ = err

	// Test with truly expired: create with 1ms, sleep, then verify
	token, err = maker.CreateToken(userID, time.Millisecond)
	if err != nil {
		t.Fatalf("CreateToken() error: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	_, err = maker.VerifyToken(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestVerifyToken_Empty(t *testing.T) {
	maker := newTestMaker(t)

	_, err := maker.VerifyToken("")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

func TestVerifyToken_Malformed(t *testing.T) {
	maker := newTestMaker(t)

	_, err := maker.VerifyToken("not.a.valid.token")
	if err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	maker := newTestMaker(t)
	userID := uuid.New()

	token, err := maker.CreateToken(userID, time.Hour)
	if err != nil {
		t.Fatalf("CreateToken() error: %v", err)
	}

	// Create a different maker with a different secret
	otherMaker, err := auth.NewJWTMaker("different-secret-key-that-is-also-32-bytes")
	if err != nil {
		t.Fatalf("NewJWTMaker() error: %v", err)
	}

	_, err = otherMaker.VerifyToken(token)
	if err == nil {
		t.Fatal("expected error when verifying with wrong secret, got nil")
	}
}

// --- Password Hashing Tests ---

func TestHashPassword_Valid(t *testing.T) {
	hash, err := auth.HashPassword("mypassword123")
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword() returned empty hash")
	}
	if hash == "mypassword123" {
		t.Fatal("HashPassword() returned plaintext password")
	}
}

func TestCheckPassword_Correct(t *testing.T) {
	password := "securepassword"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if !auth.CheckPassword(password, hash) {
		t.Error("CheckPassword() returned false for correct password")
	}
}

func TestCheckPassword_Wrong(t *testing.T) {
	hash, err := auth.HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if auth.CheckPassword("wrongpassword", hash) {
		t.Error("CheckPassword() returned true for wrong password")
	}
}

func TestHashPassword_DifferentHashes(t *testing.T) {
	password := "samepassword"
	hash1, _ := auth.HashPassword(password)
	hash2, _ := auth.HashPassword(password)

	if hash1 == hash2 {
		t.Error("HashPassword() produced identical hashes for same input (bcrypt should salt)")
	}
}

// --- Context Helpers Tests ---

func TestSetGetUserFromContext(t *testing.T) {
	userID := uuid.New()
	ctx := auth.SetUserInContext(t.Context(), userID)

	got, ok := auth.GetUserFromContext(ctx)
	if !ok {
		t.Fatal("GetUserFromContext() returned ok=false")
	}
	if got != userID {
		t.Errorf("GetUserFromContext() = %v, want %v", got, userID)
	}
}

func TestGetUserFromContext_Missing(t *testing.T) {
	_, ok := auth.GetUserFromContext(t.Context())
	if ok {
		t.Error("GetUserFromContext() returned ok=true for empty context")
	}
}

func TestGetUserFromContext_NilContext(t *testing.T) {
	_, ok := auth.GetUserFromContext(nil)
	if ok {
		t.Error("GetUserFromContext(nil) returned ok=true")
	}
}

// --- ExtractTokenFromCookie Tests ---

func TestExtractTokenFromCookie_Valid(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "my-token"})

	token, err := auth.ExtractTokenFromCookie(req)
	if err != nil {
		t.Fatalf("ExtractTokenFromCookie() error: %v", err)
	}
	if token != "my-token" {
		t.Errorf("ExtractTokenFromCookie() = %q, want %q", token, "my-token")
	}
}

func TestExtractTokenFromCookie_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := auth.ExtractTokenFromCookie(req)
	if err == nil {
		t.Fatal("expected error for missing cookie, got nil")
	}
}

func TestExtractTokenFromCookie_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: ""})

	_, err := auth.ExtractTokenFromCookie(req)
	if err == nil {
		t.Fatal("expected error for empty cookie value, got nil")
	}
}

// --- Middleware Tests ---

func TestMiddleware_ValidToken(t *testing.T) {
	maker := newTestMaker(t)
	userID := uuid.New()

	token, _ := maker.CreateToken(userID, time.Hour)

	// Handler that checks context for userID
	var gotUserID uuid.UUID
	var gotOK bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, gotOK = auth.GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.Middleware(maker)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !gotOK {
		t.Fatal("middleware did not set userID in context")
	}
	if gotUserID != userID {
		t.Errorf("context userID = %v, want %v", gotUserID, userID)
	}
}

func TestMiddleware_NoCookie(t *testing.T) {
	maker := newTestMaker(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called")
	})

	handler := auth.Middleware(maker)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	maker := newTestMaker(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called")
	})

	handler := auth.Middleware(maker)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "invalid-token"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	maker := newTestMaker(t)
	userID := uuid.New()

	token, _ := maker.CreateToken(userID, time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called for expired token")
	})

	handler := auth.Middleware(maker)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
