package middleware

import (
	"chat-v2/internal/auth"
	"chat-v2/internal/pkg/logger"
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	redisLimiter *redis_rate.Limiter
	memLimiters  sync.Map
	limit        int
	window       time.Duration
	keyPrefix    string
}

func NewRateLimiter(redis *goredis.Client, keyPrefix string, limit int, window time.Duration) *RateLimiter {
	var redisLimiter *redis_rate.Limiter
	if redis != nil {
		redisLimiter = redis_rate.NewLimiter(redis)
	}

	return &RateLimiter{
		redisLimiter: redisLimiter,
		limit:        limit,
		window:       window,
		keyPrefix:    keyPrefix,
	}
}

func RateLimit(redis *goredis.Client, keyPrefix string, limit int, window time.Duration) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(redis, keyPrefix, limit, window)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identifier := getClientIdentifier(r)
			key := limiter.keyPrefix + ":" + identifier

			result := limiter.Allow(r.Context(), key)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))

			if !result.Allowed {
				retryAfter := int(result.RetryAfter.Seconds())
				retryAfter = max(retryAfter, 1) // Ensure at least 1 second

				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				logger.Warn("Rate limit exceeded", "key", key)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type RateLimitResult struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
}

func (l *RateLimiter) Allow(ctx context.Context, key string) RateLimitResult {
	// Try Redis first
	if l.redisLimiter != nil {
		result, err := l.redisLimiter.Allow(ctx, key, redis_rate.Limit{
			Rate:   l.limit,
			Burst:  l.limit,
			Period: l.window,
		})
		if err == nil {
			return RateLimitResult{
				Allowed:    result.Allowed > 0,
				Limit:      l.limit,
				Remaining:  max(0, result.Remaining),
				RetryAfter: result.RetryAfter,
			}
		}
		logger.Warn("Redis rate limit failed, falling back to memory", "error", err)
	}

	// Memory fallback using token bucket
	return l.allowMemory(key)
}

func (l *RateLimiter) allowMemory(key string) RateLimitResult {
	// Create rate based on limit/window
	r := rate.Limit(float64(l.limit) / l.window.Seconds())

	limiterObj, _ := l.memLimiters.LoadOrStore(key, rate.NewLimiter(r, l.limit))
	memLimiter := limiterObj.(*rate.Limiter)

	if memLimiter.Allow() {
		// Approximate remaining tokens
		tokens := int(memLimiter.Tokens())
		return RateLimitResult{
			Allowed:   true,
			Limit:     l.limit,
			Remaining: tokens,
		}
	}

	return RateLimitResult{
		Allowed:    false,
		Limit:      l.limit,
		Remaining:  0,
		RetryAfter: l.window,
	}
}

func getClientIdentifier(r *http.Request) string {
	if userID, ok := auth.GetUserFromContext(r.Context()); ok && userID != uuid.Nil {
		return userID.String()
	}

	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
