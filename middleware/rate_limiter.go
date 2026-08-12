package middleware

import (
	"chat-v2/helper"
	"chat-v2/logger"
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// RateLimiter holds the configuration and client references for rate limiting.
type RateLimiter struct {
	redisClient *redis.Client
	memLimiters sync.Map // map[string]*rate.Limiter for in-memory fallback
	rate        rate.Limit
	burst       int
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter(rClient *redis.Client, requestsPerSec float64, burst int) *RateLimiter {
	return &RateLimiter{
		redisClient: rClient,
		rate:        rate.Limit(requestsPerSec),
		burst:       burst,
	}
}

// LimitMiddleware creates an HTTP middleware that limits requests by key using Redis Sliding Window
// or falls back to an in-memory Token Bucket if Redis is unavailable.
// keyPrefix identifies the scope (e.g., "login", "signup", "api").
// limit is max requests allowed within windowDuration.
func LimitMiddleware(rClient *redis.Client, keyPrefix string, limit int, windowDuration time.Duration) func(http.Handler) http.Handler {
	inMemLimiter := NewRateLimiter(rClient, float64(limit)/windowDuration.Seconds(), limit)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identifier := getClientIdentifier(r)
			redisKey := fmt.Sprintf("rl:%s:%s", keyPrefix, identifier)

			allowed, count, remaining, err := checkRateLimit(r.Context(), rClient, inMemLimiter, redisKey, identifier, limit, windowDuration)
			if err != nil && logger.Log != nil {
				logger.Log.Warn("Rate limit check error, allowing request", "error", err, "key", redisKey)
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				retryAfterSecs := int(windowDuration.Seconds())
				if retryAfterSecs < 1 {
					retryAfterSecs = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSecs))
				if logger.Log != nil {
					logger.Log.Warn("Rate limit exceeded", "key", redisKey, "count", count, "limit", limit)
				}
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func checkRateLimit(ctx context.Context, rClient *redis.Client, fallback *RateLimiter, key, identifier string, limit int, windowDuration time.Duration) (allowed bool, currentCount int, remaining int, err error) {
	if rClient != nil {
		nowMs := time.Now().UnixNano() / int64(time.Millisecond)
		windowStartMs := nowMs - windowDuration.Milliseconds()
		member := fmt.Sprintf("%d-%s", nowMs, uuid.NewString()[:8])

		pipe := rClient.Pipeline()
		pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStartMs, 10))
		pipe.ZAdd(ctx, key, redis.Z{Score: float64(nowMs), Member: member})
		countCmd := pipe.ZCard(ctx, key)
		pipe.Expire(ctx, key, windowDuration+time.Second)

		_, pipeErr := pipe.Exec(ctx)
		if pipeErr == nil {
			count := int(countCmd.Val())
			rem := limit - count
			if rem < 0 {
				rem = 0
			}
			return count <= limit, count, rem, nil
		}
		if logger.Log != nil {
			logger.Log.Warn("Redis rate limit pipeline failed, using memory fallback", "error", pipeErr)
		}
	}

	// Memory fallback
	limiterObj, _ := fallback.memLimiters.LoadOrStore(identifier, rate.NewLimiter(fallback.rate, fallback.burst))
	limiter := limiterObj.(*rate.Limiter)

	if limiter.Allow() {
		return true, 1, limit - 1, nil
	}
	return false, limit, 0, nil
}

// getClientIdentifier extracts User ID if authenticated; otherwise falls back to IP address.
func getClientIdentifier(r *http.Request) string {
	if userID, ok := helper.GetUserFromContext(r.Context()); ok && userID != uuid.Nil {
		return userID.String()
	}

	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
