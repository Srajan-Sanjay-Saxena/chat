package Middleware

import (
	"chat-v2/helper"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTMiddleware_ProtectedEndpointValidation(t *testing.T) {
	maker, err := helper.NewJWTMaker("chshgif-sjrbn0-snekc-akfknce-afrnlkj")
	if err != nil {
		t.Fatalf("new jwt maker: %v", err)
	}

	userID := uuid.New()
	token, err := maker.CreateToken(userID, time.Hour)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	dummyProtectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			t.Fatalf("expected user id in request context")
		}
		if gotUserID != userID {
			t.Fatalf("unexpected user id in context: got=%s want=%s", gotUserID, userID)
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := JWTMiddleware(maker)
	handler := middleware(dummyProtectedHandler)

	t.Run("rejects missing authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusUnauthorized)
		}
		if !strings.Contains(rec.Body.String(), "authorization header is missing") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("allows valid bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusOK)
		}
	})
}
