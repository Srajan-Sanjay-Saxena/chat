package handler

import (
	"chat-v2/db"
	"chat-v2/helper"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"chat-v2/middleware"
	"github.com/google/uuid"
	"chat-v2/logger"
)

type stubUserSearchRepo struct {
	users []*db.User
	err   error
}

func (s stubUserSearchRepo) SearchUsers(_ context.Context, _ string, _ int) ([]*db.User, error) {
	return s.users, s.err
}

func TestValidateUserSearchQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{name: "empty", query: "", wantErr: true},
		{name: "invalid chars", query: "ali$ce", wantErr: true},
		{name: "too long", query: "abcdefghijklmnopqrstu", wantErr: true},
		{name: "ok", query: "alice_1", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUserSearchQuery(tt.query)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for query %q", tt.query)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for query %q: %v", tt.query, err)
			}
		})
	}
}

func TestUserSearchHandler_ValidationAndRateLimit(t *testing.T) {
	resetUserSearchLimiter()
	logger.TestInit()
	maker, err := helper.NewJWTMaker("chshgif-sjrbn0-snekc-akfknce-afrnlkj")
	if err != nil {
		t.Fatalf("new jwt maker: %v", err)
	}
	token, err := maker.CreateToken(uuid.New(), time.Hour)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	authMiddleware := middleware.JWTMiddleware(maker)

	handler := authMiddleware(UserSearchHandler(stubUserSearchRepo{
		users: []*db.User{{ID: uuid.New(), Username: "alice"}},
	}))

	t.Run("rejects invalid query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/search?q=ali$ce", nil)
		req.Header.Add("Cookie", "access_token="+token)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("rejects invalid limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/search?q=alice&limit=0", nil)
		req.Header.Add("Cookie", "access_token="+token)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("allows search and rate limits after threshold", func(t *testing.T) {
		resetUserSearchLimiter()
		for i := 0; i < userSearchMaxPerWindow; i++ {
			req := httptest.NewRequest(http.MethodGet, "/users/search?q=alice&limit=5", nil)
			req.Header.Add("Cookie", "access_token="+token)
			req.RemoteAddr = "127.0.0.1:12345"
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected success before rate limit, got=%d body=%s", rec.Code, rec.Body.String())
			}
			var payload map[string][]*db.User
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
		}

		req := httptest.NewRequest(http.MethodGet, "/users/search?q=alice&limit=5", nil)
		req.Header.Add("Cookie", "access_token="+token)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("unexpected status after rate limit: got=%d want=%d", rec.Code, http.StatusTooManyRequests)
		}
	})
}
