package handler

import (
	"chat-v2/db"
	"chat-v2/helper"
	"chat-v2/logger"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type userSearcher interface {
	SearchUsers(ctx context.Context, q string, limit int) ([]*db.User, error)
}

// UserSearchHandler returns an endpoint to search users by username prefix.
// GET /users/search?q=alice&limit=10
// It will return a JSON response with the following structure:
// {
//     "users": [
//         {
//             "id": "uuid-of-user",
//             "username": "user's username"
//         },
//         ...
//     ]
// }
// The handler validates the request, checks if the user is authenticated, and then performs a search in the database using the repository layer. It also includes rate limiting to prevent abuse of the search functionality.
func UserSearchHandler(repo userSearcher) http.Handler {

	if logger.Log == nil {
		panic("logger is nil")
	}

	if repo == nil {
		logger.Log.Error("UserSearchHandler initialization failed: repository is nil")
		panic("repository cannot be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		if !allowUserSearchRequest(r) {
			writeJSONError(w, http.StatusTooManyRequests, "Too many user search requests")
			return
		}

		_, ok := helper.GetUserFromContext(r.Context())
		if !ok {
			logger.Log.Error("Failed to get user from context in UserSearchHandler")
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized: user not found in context")
			return
		}

		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if err := validateUserSearchQuery(q); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		limit := 10
		if l := r.URL.Query().Get("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil {
				limit = v
			} else {
				writeJSONError(w, http.StatusBadRequest, "limit must be an integer")
				return
			}
		}
		if err := validateUserSearchLimit(limit); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		users, err := repo.SearchUsers(r.Context(), q, limit)
		if err != nil {
			logger.Log.Error("Failed to search users", "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Failed to search users")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"users": users})
	})
}

func validateUserSearchQuery(q string) error {
	if q == "" {
		return ErrUserSearchQueryRequired
	}
	if utf8.RuneCountInString(q) > 20 {
		return ErrUserSearchQueryTooLong
	}
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			continue
		}
		return ErrUserSearchQueryInvalid
	}
	return nil
}

func validateUserSearchLimit(limit int) error {
	if limit < 1 {
		return ErrUserSearchLimitTooSmall
	}
	if limit > 25 {
		return ErrUserSearchLimitTooLarge
	}
	return nil
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}
	if xreal := strings.TrimSpace(r.Header.Get("X-Real-IP")); xreal != "" {
		return xreal
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
