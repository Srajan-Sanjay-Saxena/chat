package middleware

import (
	"chat-v2/internal/auth"
	"chat-v2/internal/pkg/logger"
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	redis       *goredis.Client
	memLimiters sync.Map
	rate        rate.Limit
	burst       int
}

func RateLimit(redis *goredis.Client, keyPrefix string, limit int, window time.Duration) func(http.Handler) http.Handler {
	limiter := &RateLimiter{
		redis: redis,
		rate:  rate.Limit(float64(limit) / window.Seconds()),
		burst: limit,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identifier := getClientIdentifier(r)
			redisKey := fmt.Sprintf("rl:%s:%s", keyPrefix, identifier)

			allowed, count, remaining := limiter.check(r.Context(), redisKey, identifier, limit, window)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				logger.Warn("Rate limit exceeded", "key", redisKey, "count", count)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (l *RateLimiter) check(ctx context.Context, key, identifier string, limit int, window time.Duration) (allowed bool, count int, remaining int) {
	if l.redis != nil {
		nowMs := time.Now().UnixNano() / int64(time.Millisecond)
		windowStartMs := nowMs - window.Milliseconds()
		member := fmt.Sprintf("%d-%s", nowMs, uuid.NewString()[:8])

		pipe := l.redis.Pipeline()
		pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStartMs, 10))
		pipe.ZAdd(ctx, key, goredis.Z{Score: float64(nowMs), Member: member})
		countCmd := pipe.ZCard(ctx, key)
		pipe.Expire(ctx, key, window+time.Second)

		if _, err := pipe.Exec(ctx); err == nil {
			count := int(countCmd.Val())
			rem := limit - count
			if rem < 0 {
				rem = 0
			}
			return count <= limit, count, rem
		}
	}

	// Memory fallback
	limiterObj, _ := l.memLimiters.LoadOrStore(identifier, rate.NewLimiter(l.rate, l.burst))
	memLimiter := limiterObj.(*rate.Limiter)

	if memLimiter.Allow() {
		return true, 1, limit - 1
	}
	return false, limit, 0
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
