package handler

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

var (
	ErrUserSearchQueryRequired = errors.New("q query parameter is required")
	ErrUserSearchQueryTooLong  = errors.New("q query parameter is too long")
	ErrUserSearchQueryInvalid  = errors.New("q query parameter contains invalid characters")
	ErrUserSearchLimitTooSmall = errors.New("limit must be at least 1")
	ErrUserSearchLimitTooLarge = errors.New("limit must be at most 25")
)

const (
	userSearchWindow       = time.Minute
	userSearchMaxPerWindow = 8
)

type searchWindowState struct {
	windowStart time.Time
	count       int
}

var userSearchLimiter = struct {
	sync.Mutex
	items map[string]*searchWindowState
}{items: make(map[string]*searchWindowState)}

func allowUserSearchRequest(r *http.Request) bool {
	key := clientIP(r)
	now := time.Now()

	userSearchLimiter.Lock()
	defer userSearchLimiter.Unlock()

	state, ok := userSearchLimiter.items[key]
	if !ok || now.Sub(state.windowStart) >= userSearchWindow {
		userSearchLimiter.items[key] = &searchWindowState{windowStart: now, count: 1}
		return true
	}

	if state.count >= userSearchMaxPerWindow {
		return false
	}

	state.count++
	return true
}

// resetUserSearchLimiter clears limiter state for tests.
func resetUserSearchLimiter() {
	userSearchLimiter.Lock()
	defer userSearchLimiter.Unlock()
	userSearchLimiter.items = make(map[string]*searchWindowState)
}
