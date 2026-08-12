package realtime

import (
	"golang.org/x/time/rate"
)

// ConnectionLimiter controls the rate of incoming frames per WebSocket client.
type ConnectionLimiter struct {
	limiter *rate.Limiter
}

// NewConnectionLimiter initializes a Token Bucket rate limiter.
// Parameters:
//   - r: Events per second allowed (rate.Limit)
//   - b: Maximum burst size (int)
func NewConnectionLimiter(r rate.Limit, b int) *ConnectionLimiter {
	return &ConnectionLimiter{
		limiter: rate.NewLimiter(r, b),
	}
}

// Allow checks whether an incoming WebSocket message frame can be processed under current rate limits.
func (cl *ConnectionLimiter) Allow() bool {
	if cl == nil || cl.limiter == nil {
		return true
	}
	return cl.limiter.Allow()
}
